package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

type ClientReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	CheckInterval time.Duration
}

// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=clients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=clients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=clients/finalizers,verbs=update
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=realms,verbs=get;update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *ClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var obj v1alpha1.Client
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&obj, finalizer) {
			safe, err := IsSafelyDeletedFromRealm(ctx, r.Client, req.Namespace, obj.Spec.RealmRef, obj.DeletionTimestamp)
			if err != nil {
				return ctrl.Result{}, err
			}
			if safe {
				log.Info("Client was successfully purged from Keycloak, removing finalizer", "clientId", obj.Spec.ClientID)
				r.Recorder.Eventf(&obj, corev1.EventTypeNormal, "SafeDelete", "Successfully purged Client '%s' from Keycloak", obj.Spec.ClientID)
				return ctrl.Result{}, removeFinalizer(ctx, r.Client, req.NamespacedName, &obj)
			}
			// Deletion is not confirmed yet: the Realm has not completed a successful sync after this CR's deletion timestamp.
			// Trigger Realm sync and requeue until the post-delete sync has been observed.
			_ = TriggerRealmSync(ctx, r.Client, req.Namespace, obj.Spec.RealmRef)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&obj, finalizer) {
		controllerutil.AddFinalizer(&obj, finalizer)
		return ctrl.Result{}, r.Update(ctx, &obj)
	}

	if err := r.sync(ctx, &obj); err != nil {
		log.Error(err, "sync failed", "clientId", obj.Spec.ClientID)
		r.Recorder.Eventf(&obj, corev1.EventTypeWarning, "SyncFailed", "Failed to delegate sync: %v", err)
		err2 := UpdateStatusWithRetry(ctx, r.Client, req.NamespacedName, &obj, func(latest *v1alpha1.Client) {
			setFailed(&latest.Status.CommonStatus, err.Error())
		})
		if err2 != nil {
			log.Error(err2, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
}

func (r *ClientReconciler) sync(ctx context.Context, obj *v1alpha1.Client) error {
	realmName := obj.Spec.RealmRef
	if realmName == "" {
		realmName = "master"
	}

	// For confidential clients, generate the secret if it doesn't exist yet,
	// so the Builder can pick it up and bundle it into the realm.json export.
	isPublic := obj.Spec.PublicClient != nil && *obj.Spec.PublicClient
	if !isPublic {
		secretName := obj.Spec.ClientID + "-secret"
		existing := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: obj.Namespace}, existing)
		if k8serrors.IsNotFound(err) {
			secretValue, err := generateRandomString(32)
			if err != nil {
				return fmt.Errorf("failed to generate client secret: %w", err)
			}
			// Generate one
			desired := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: obj.Namespace,
				},
				StringData: map[string]string{
					"CLIENT_ID":     obj.Spec.ClientID,
					"CLIENT_SECRET": secretValue,
				},
			}
			if err := r.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create client secret: %w", err)
			}
		} else if err != nil {
			return err
		}
	}

	if err := TriggerRealmSync(ctx, r.Client, obj.Namespace, realmName); err != nil {
		return err
	}
	setReady(&obj.Status.CommonStatus, obj.Spec.ClientID, "Delegated to Realm Sync")
	return r.Status().Update(ctx, obj)
}

func (r *ClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Client{}).
		Complete(r)
}

func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:length], nil
}
