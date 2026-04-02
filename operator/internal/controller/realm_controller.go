package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
	"github.com/opendefensecloud/keycloak-bundle/operator/internal/wrapper"
)

type RealmReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Runner        *wrapper.JobRunner
	Recorder      record.EventRecorder
	CheckInterval time.Duration
}

// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=realms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=realms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=realms/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *RealmReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var obj v1alpha1.Realm
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&obj, finalizer) {
			log.Info("deletion requested — preserving realm in Keycloak, removing finalizer",
				"realmName", obj.Spec.RealmName)
			r.Recorder.Eventf(&obj, corev1.EventTypeNormal, "SafeDelete", "Preserving realm '%s' in Keycloak but removing K8s management", obj.Spec.RealmName)
			if err := removeFinalizer(ctx, r.Client, req.NamespacedName, &obj); err != nil {
				return ctrl.Result{}, err
			}
			// We do NOT trigger sync on realm deletion, as the CLI tool doesn't delete realms.
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&obj, finalizer) {
		controllerutil.AddFinalizer(&obj, finalizer)
		return ctrl.Result{}, r.Update(ctx, &obj)
	}

	if err := r.sync(ctx, &obj); err != nil {
		log.Error(err, "sync failed", "realmName", obj.Spec.RealmName)
		r.Recorder.Eventf(&obj, corev1.EventTypeWarning, "SyncFailed", "Failed to sync Realm config via Config-CLI: %v", err)
		setFailed(&obj.Status.CommonStatus, err.Error())
		if err2 := r.Status().Update(ctx, &obj); err2 != nil {
			log.Error(err2, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
}

func (r *RealmReconciler) sync(ctx context.Context, obj *v1alpha1.Realm) error {
	realmName := obj.Spec.RealmName
	if realmName == "" {
		realmName = "master"
	}
	export, err := wrapper.BuildRealmExport(ctx, r.Client, obj.Namespace, *obj)
	if err != nil {
		return err
	}

	if err := r.Runner.SyncRealm(ctx, obj, export, r.Scheme); err != nil {
		return err
	}

	// Once successfully synced, we can remove the trigger annotation if it exists
	if obj.Annotations != nil {
		if _, ok := obj.Annotations[SyncRequestedAnnotation]; ok {
			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Client.Get(ctx, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj); err != nil {
					return err
				}
				delete(obj.Annotations, SyncRequestedAnnotation)
				return r.Client.Update(ctx, obj)
			}); err != nil {
				return err
			}
			// Return here to avoid Conflict. The update triggers an immediate requeue.
			// The next run will skip job creation (no-op) and proceed to status update.
			return nil
		}
	}

	r.Recorder.Eventf(obj, corev1.EventTypeNormal, "SyncSuccessful", "Successfully synchronized full Keycloak Realm '%s'", realmName)
	setReady(&obj.Status.CommonStatus, realmName, "Synced successfully")
	return r.Status().Update(ctx, obj)
}

func (r *RealmReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Realm{}).
		Complete(r)
}
