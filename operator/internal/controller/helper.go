package controller

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

const finalizer = "keycloak.opendefense.cloud/cleanup"

// Leader election uses coordination.k8s.io Lease objects.
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// removeFinalizer removes the standard finalizer from obj using RetryOnConflict.
// It re-fetches the latest ResourceVersion before each attempt to prevent stale-write conflicts
// in the deletion path, where a conflict would leave the finalizer permanently stuck.
func removeFinalizer(ctx context.Context, c client.Client, key types.NamespacedName, obj client.Object) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := c.Get(ctx, key, obj); err != nil {
			return err
		}
		controllerutil.RemoveFinalizer(obj, finalizer)
		return c.Update(ctx, obj)
	})
}

func setReady(s *v1alpha1.CommonStatus, id, msg string) {
	now := metav1.Now()
	s.Ready = true
	s.KeycloakID = id
	s.Message = msg
	s.LastSyncTime = &now
	s.Conditions = []metav1.Condition{{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "Synced",
		Message:            msg,
	}}
}

func setFailed(s *v1alpha1.CommonStatus, msg string) {
	now := metav1.Now()
	s.Ready = false
	s.Message = msg
	s.LastSyncTime = &now
	s.Conditions = []metav1.Condition{{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             "SyncFailed",
		Message:            msg,
	}}
}

// requeueDelay is how long to wait before retrying after a transient error.
const requeueDelay = 30 * time.Second

const SyncRequestedAnnotation = "keycloak.opendefense.cloud/sync-requested"

// TriggerRealmSync annotates the Realm to wake up the realm_controller.
func TriggerRealmSync(ctx context.Context, c client.Client, namespace, realmName string) error {
	if realmName == "" {
		realmName = "master"
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var realm v1alpha1.Realm
		if err := c.Get(ctx, types.NamespacedName{Name: realmName, Namespace: namespace}, &realm); err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("target Realm '%s' not found in namespace '%s'", realmName, namespace)
			}
			return err
		}

		if realm.Annotations == nil {
			realm.Annotations = make(map[string]string)
		}
		realm.Annotations[SyncRequestedAnnotation] = time.Now().Format(time.RFC3339Nano)

		return c.Update(ctx, &realm)
	})
}

// IsSafelyDeletedFromRealm checks if the Realm has completed a successful sync AFTER the child's deletion timestamp.
func IsSafelyDeletedFromRealm(ctx context.Context, c client.Client, namespace, realmName string, deletionTime *metav1.Time) (bool, error) {
	if deletionTime == nil {
		return false, nil // Not deleting
	}
	if realmName == "" {
		realmName = "master"
	}
	var realm v1alpha1.Realm
	if err := c.Get(ctx, types.NamespacedName{Name: realmName, Namespace: namespace}, &realm); err != nil {
		// If realm is gone, the child is safely deleted by Keycloak cascade or K8s cascade.
		return true, client.IgnoreNotFound(err)
	}

	// For strict auditing: the Realm must have completed a successful Job *after* we asked for deletion.
	if realm.Status.Ready && realm.Status.LastSyncTime != nil && realm.Status.LastSyncTime.After(deletionTime.Time) {
		return true, nil
	}

	return false, nil
}
