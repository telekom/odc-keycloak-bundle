package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;delete
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

	result, err := r.sync(ctx, &obj)
	if err != nil {
		log.Error(err, "sync failed", "realmName", obj.Spec.RealmName)
		r.Recorder.Eventf(&obj, corev1.EventTypeWarning, "SyncFailed", "Failed to sync Realm config via Config-CLI: %v", err)
		err2 := UpdateStatusWithRetry(ctx, r.Client, req.NamespacedName, &obj, func(latest *v1alpha1.Realm) {
			latest.Status.ObservedGeneration = latest.Generation
			setFailed(&latest.Status.CommonStatus, err.Error())
		})
		if err2 != nil {
			log.Error(err2, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}
	if result.RequeueAfter > 0 {
		return result, nil
	}

	return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
}

func (r *RealmReconciler) sync(ctx context.Context, obj *v1alpha1.Realm) (ctrl.Result, error) {
	realmName := obj.Spec.RealmName
	if realmName == "" {
		return ctrl.Result{}, fmt.Errorf("realm %q has empty spec.realmName", obj.Name)
	}
	export, err := wrapper.BuildRealmExport(ctx, r.Client, obj.Namespace, *obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	jobName, err := r.Runner.SyncRealm(ctx, obj, export, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	// If a new Job was spawned, record it in status immediately as Pending.
	if jobName != "" {
		if err := UpdateStatusWithRetry(ctx, r.Client, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj, func(latest *v1alpha1.Realm) {
			latest.Status.ObservedGeneration = latest.Generation
			setPending(&latest.Status.CommonStatus, "Config-CLI Job running")
			latest.Status.ActiveJobName = jobName
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	activeJobName := jobName
	if activeJobName == "" {
		activeJobName = obj.Status.ActiveJobName
	}

	// Clean up terminal Jobs older than 10 minutes owned by this Realm.
	if cleanErr := r.cleanupTerminalJobs(ctx, obj, activeJobName); cleanErr != nil {
		ctrl.LoggerFrom(ctx).Error(cleanErr, "failed to clean up terminal Jobs", "realmName", realmName)
	}

	// Remove trigger annotation if present; the resulting requeue will then observe the Job.
	if obj.Annotations != nil {
		if _, ok := obj.Annotations[SyncRequestedAnnotation]; ok {
			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj); err != nil {
					return err
				}
				delete(obj.Annotations, SyncRequestedAnnotation)
				return r.Update(ctx, obj)
			}); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: jobObserveInterval}, nil
		}
	}

	// Phase 2: observe the active Job to determine final CR status.
	if activeJobName == "" {
		// No Job ever spawned and status is already Ready — nothing to do.
		if obj.Status.Ready {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, UpdateStatusWithRetry(ctx, r.Client, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj, func(latest *v1alpha1.Realm) {
			latest.Status.ObservedGeneration = latest.Generation
			setPending(&latest.Status.CommonStatus, "No active Config-CLI Job tracked; waiting for next sync")
		})
	}

	return r.observeJob(ctx, obj, activeJobName, realmName)
}

// observeJob fetches the named Job and updates the Realm status based on its outcome.
func (r *RealmReconciler) observeJob(ctx context.Context, obj *v1alpha1.Realm, jobName, realmName string) (ctrl.Result, error) {
	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: obj.Namespace}, &job); err != nil {
		if apierrors.IsNotFound(err) {
			// Job already gone. If status is already terminal and ready, leave it.
			if obj.Status.Ready {
				return ctrl.Result{}, nil
			}
			msg := "active config-cli Job disappeared before a terminal result was observed"
			return ctrl.Result{}, UpdateStatusWithRetry(ctx, r.Client, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj, func(latest *v1alpha1.Realm) {
				latest.Status.ObservedGeneration = latest.Generation
				setFailed(&latest.Status.CommonStatus, msg)
			})
		}
		return ctrl.Result{}, err
	}

	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, "SyncSuccessful", "Successfully synchronized Keycloak Realm '%s'", realmName)
			return ctrl.Result{}, UpdateStatusWithRetry(ctx, r.Client, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj, func(latest *v1alpha1.Realm) {
				latest.Status.ObservedGeneration = latest.Generation
				setReady(&latest.Status.CommonStatus, realmName, "Synced successfully")
				latest.Status.ActiveJobName = ""
			})
		case batchv1.JobFailed:
			msg := c.Message
			if msg == "" {
				msg = "config-cli Job failed"
			}
			return ctrl.Result{}, UpdateStatusWithRetry(ctx, r.Client, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj, func(latest *v1alpha1.Realm) {
				latest.Status.ObservedGeneration = latest.Generation
				setFailed(&latest.Status.CommonStatus, msg)
				latest.Status.ActiveJobName = jobName
			})
		}
	}

	// Job still running — update status and requeue to poll again.
	if err := UpdateStatusWithRetry(ctx, r.Client, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj, func(latest *v1alpha1.Realm) {
		latest.Status.ObservedGeneration = latest.Generation
		setPending(&latest.Status.CommonStatus, "Config-CLI Job running")
	}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: jobObserveInterval}, nil
}

func (r *RealmReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Realm{}).
		Complete(r)
}

// cleanupTerminalJobs deletes Jobs owned by realm that have reached a terminal state
// (Complete or Failed) and were created more than 10 minutes ago.
func (r *RealmReconciler) cleanupTerminalJobs(ctx context.Context, realm *v1alpha1.Realm, activeJobName string) error {
	var jobList batchv1.JobList
	if err := r.List(ctx, &jobList,
		client.InNamespace(realm.Namespace),
		client.MatchingLabels{"app": wrapper.LabelApp},
	); err != nil {
		return err
	}

	cutoff := time.Now().Add(-10 * time.Minute)
	for i := range jobList.Items {
		job := &jobList.Items[i]
		if job.Name == activeJobName {
			continue
		}
		if !isOwnedByRealm(job, realm) || !isJobTerminal(job) || job.CreationTimestamp.After(cutoff) {
			continue
		}
		propagation := metav1.DeletePropagationBackground
		if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func isOwnedByRealm(job *batchv1.Job, realm *v1alpha1.Realm) bool {
	for _, ref := range job.OwnerReferences {
		if ref.UID == realm.UID {
			return true
		}
	}
	return false
}

func isJobTerminal(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Status == corev1.ConditionTrue &&
			(c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) {
			return true
		}
	}
	return false
}
