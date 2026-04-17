package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

type GroupReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	CheckInterval time.Duration
}

// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=groups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=groups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=groups/finalizers,verbs=update
// +kubebuilder:rbac:groups=keycloak.opendefense.cloud,resources=realms,verbs=get;update

func (r *GroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var obj v1alpha1.Group
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
				log.Info("Group was successfully purged from Keycloak, removing finalizer", "name", obj.Spec.Name)
				r.Recorder.Eventf(&obj, corev1.EventTypeNormal, "SafeDelete", "Successfully purged Group '%s' from Keycloak", obj.Spec.Name)
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
		log.Error(err, "sync failed", "groupName", obj.Spec.Name)
		r.Recorder.Eventf(&obj, corev1.EventTypeWarning, "SyncFailed", "Failed to delegate sync: %v", err)
		err2 := UpdateStatusWithRetry(ctx, r.Client, req.NamespacedName, &obj, func(latest *v1alpha1.Group) {
			setFailed(&latest.Status.CommonStatus, err.Error())
		})
		if err2 != nil {
			log.Error(err2, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}

	return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
}

func (r *GroupReconciler) sync(ctx context.Context, obj *v1alpha1.Group) error {
	realmName := obj.Spec.RealmRef
	if realmName == "" {
		realmName = "master"
	}
	if err := TriggerRealmSync(ctx, r.Client, obj.Namespace, realmName); err != nil {
		return err
	}
	setReady(&obj.Status.CommonStatus, obj.Spec.Name, "Delegated to Realm Sync")
	return r.Status().Update(ctx, obj)
}

func (r *GroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Group{}).
		Complete(r)
}
