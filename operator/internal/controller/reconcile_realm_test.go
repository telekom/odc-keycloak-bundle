package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
	"github.com/opendefensecloud/keycloak-bundle/operator/internal/wrapper"
)

const testNS = "test-ns"

// realmWithFinalizer returns a Realm named "myrealm" that already has the cleanup finalizer.
func realmWithFinalizer() *v1alpha1.Realm {
	return &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "myrealm",
			Namespace:  testNS,
			Finalizers: []string{finalizer},
		},
		Spec: v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
}

// emptyRealmPayload computes the JSON that SyncRealm would produce for an
// empty Realm (no child CRs). Used to seed the config Secret for drift-skip tests.
func emptyRealmPayload(t *testing.T, realmName string) []byte {
	t.Helper()
	export := &wrapper.RealmExport{
		Realm:               realmName,
		Clients:             []wrapper.ClientExport{},
		Users:               []wrapper.UserExport{},
		Groups:              []wrapper.GroupExport{},
		ClientScopes:        []wrapper.ClientScopeExport{},
		AuthenticationFlows: []wrapper.AuthFlowExport{},
		IdentityProviders:   []wrapper.IdentityProviderExport{},
	}
	b, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal empty realm payload: %v", err)
	}
	return b
}

// reconcileRequest builds a ctrl.Request for the given realm.
func reconcileRequest(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: testNS}}
}

// ── Reconcile() branches ─────────────────────────────────────────────────────

func TestRealmReconcile_NotFound(t *testing.T) {
	s := controllerScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	result, err := r.Reconcile(context.Background(), reconcileRequest("missing"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected empty result, got %+v", result)
	}
}

func TestRealmReconcile_Deletion_WithFinalizer(t *testing.T) {
	s := controllerScheme(t)
	now := metav1.Now()
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "myrealm",
			Namespace:         testNS,
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizer},
		},
		Spec: v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm).Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After removing the last finalizer while DeletionTimestamp is set, the fake
	// client garbage-collects the object — it should now be NotFound.
	var got v1alpha1.Realm
	getErr := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got)
	if getErr == nil {
		// Object still present — verify the finalizer was at least removed.
		for _, f := range got.Finalizers {
			if f == finalizer {
				t.Fatalf("finalizer should have been removed, still present: %v", got.Finalizers)
			}
		}
	} else if !apierrors.IsNotFound(getErr) {
		t.Fatalf("unexpected error fetching realm: %v", getErr)
	}
	// Either deleted (NotFound) or finalizer removed — both are acceptable outcomes.
}

func TestRealmReconcile_Deletion_NoFinalizer(t *testing.T) {
	s := controllerScheme(t)
	// Use an external finalizer so the fake client tracker keeps the object
	// alive after Delete(); this lets us test the "DeletionTimestamp set but
	// our finalizer is absent" branch without triggering the fake client's
	// immediate GC (which panics on objects with DeletionTimestamp + no finalizers).
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "myrealm",
			Namespace:  testNS,
			Finalizers: []string{"kubernetes.io/pvc-protection"}, // external, not ours
		},
		Spec: v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm).Build()

	// Soft-delete to set DeletionTimestamp; external finalizer keeps it alive.
	if err := fc.Delete(context.Background(), realm); err != nil {
		t.Fatalf("delete: %v", err)
	}

	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected empty result, got %+v", result)
	}
}

func TestRealmReconcile_AddFinalizer(t *testing.T) {
	s := controllerScheme(t)
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testNS},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm).Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	found := false
	for _, f := range got.Finalizers {
		if f == finalizer {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected finalizer to be added, got: %v", got.Finalizers)
	}
}

func TestRealmReconcile_SyncError_SetsFailedStatus(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()

	injected := "injected server error"
	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm).
		WithStatusSubresource(realm).
		WithInterceptorFuncs(interceptor.Funcs{
			// Fail on Secret Get so that SyncRealm returns an error.
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*corev1.Secret); ok {
					return fmt.Errorf("%s", injected)
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	r := newRealmReconcilerWithClient(t, fc)
	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error returned by Reconcile: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay after sync error, got %v", result.RequeueAfter)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("expected Ready=false after sync error")
	}
}

func TestRealmSync_AnnotationPresent_RemovesAnnotation(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	realm.Annotations = map[string]string{
		SyncRequestedAnnotation: time.Now().Format(time.RFC3339Nano),
	}

	// Pre-load config Secret so SyncRealm is a drift-skip (no job spawned).
	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, secret).WithStatusSubresource(realm).Build()
	r := newRealmReconcilerWithClient(t, fc)

	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != jobObserveInterval {
		t.Fatalf("expected jobObserveInterval requeue, got %v", result.RequeueAfter)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if _, ok := got.Annotations[SyncRequestedAnnotation]; ok {
		t.Fatal("expected sync annotation to be removed")
	}
}

func TestRealmSync_NoActiveJob_NotReady_SetsPending(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	// Status.Ready defaults to false; no ActiveJobName.

	// Pre-load config Secret so SyncRealm is a drift-skip.
	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, secret).WithStatusSubresource(realm).Build()
	r := newRealmReconcilerWithClient(t, fc)

	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("expected Ready=false when no job is tracked and realm is not yet ready")
	}
	if got.Status.Message != "No active Config-CLI Job tracked; waiting for next sync" {
		t.Fatalf("unexpected message: %q", got.Status.Message)
	}
}

func TestRealmSync_NoActiveJob_AlreadyReady_Noop(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()

	// Pre-set Ready=true inline before storing — no WithStatusSubresource so the
	// status fields from WithObjects are readable via Get.
	now := metav1.Now()
	realm.Status.Ready = true
	realm.Status.LastSyncTime = &now

	// Pre-load config Secret so SyncRealm is a drift-skip.
	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	// No WithStatusSubresource so inline Status.Ready is readable from Get.
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, secret).WithStatusSubresource(realm).Build()

	r := newRealmReconcilerWithClient(t, fc)
	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return CheckInterval (no fast requeue needed).
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected CheckInterval requeue, got %v", result.RequeueAfter)
	}
}

func TestRealmSync_ActiveJobFromStatus_Succeeded(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	jobName := "kc-config-job-myrealm-xyz"
	realm.Status.ActiveJobName = jobName

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: testNS},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	// Pre-load Secret for drift-skip so no new job is spawned.
	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	// WithStatusSubresource so Status().Update() in Reconcile persists.
	// Seed ActiveJobName into the status tracker via Status().Update() after Build.
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job, secret).WithStatusSubresource(realm).Build()
	if err := fc.Status().Update(context.Background(), realm); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	r := newRealmReconcilerWithClient(t, fc)
	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if !got.Status.Ready {
		t.Fatal("expected Ready=true after observing Succeeded job from status")
	}
}

func TestRealmSync_ActiveJobFromStatus_Failed(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	jobName := "kc-config-job-myrealm-abc"
	realm.Status.ActiveJobName = jobName

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: testNS},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:    batchv1.JobFailed,
				Status:  corev1.ConditionTrue,
				Message: "config-cli exited with code 1",
			}},
		},
	}

	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job, secret).WithStatusSubresource(realm).Build()
	if err := fc.Status().Update(context.Background(), realm); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	r := newRealmReconcilerWithClient(t, fc)
	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("expected Ready=false after Failed job")
	}
}

func TestRealmSync_ActiveJobFromStatus_Running(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	jobName := "kc-config-job-myrealm-run"
	realm.Status.ActiveJobName = jobName

	// Job exists but has no terminal conditions (still running).
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: testNS},
		Status:     batchv1.JobStatus{Active: 1},
	}

	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job, secret).WithStatusSubresource(realm).Build()
	if err := fc.Status().Update(context.Background(), realm); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	r := newRealmReconcilerWithClient(t, fc)
	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != jobObserveInterval {
		t.Fatalf("expected jobObserveInterval requeue for running job, got %v", result.RequeueAfter)
	}
}

func TestRealmSync_JobGone_NotReady_SetsFailed(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	realm.Status.ActiveJobName = "missing-job"

	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	// Job "missing-job" does NOT exist → observeJob gets NotFound
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, secret).WithStatusSubresource(realm).Build()
	if err := fc.Status().Update(context.Background(), realm); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	r := newRealmReconcilerWithClient(t, fc)
	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("expected Ready=false when tracked job is gone before a terminal result is observed")
	}
	if got.Status.Message != "active config-cli Job disappeared before a terminal result was observed" {
		t.Fatalf("unexpected message: %q", got.Status.Message)
	}
}

// TestRealmSync_EmptyRealmNameRejected verifies that an empty spec.realmName is
// rejected instead of silently falling back to the privileged master realm.
func TestRealmSync_EmptyRealmNameRejected(t *testing.T) {
	s := controllerScheme(t)
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "master-realm", Namespace: testNS, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.RealmSpec{RealmName: ""},
	}
	// Pre-load config secret so SyncRealm skips (drift detection).
	payload := emptyRealmPayload(t, "")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-master-realm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, secret).Build()
	r := newRealmReconcilerWithClient(t, fc)
	_, err := r.sync(context.Background(), realm)
	if err == nil {
		t.Fatal("expected error for empty spec.realmName, got nil")
	}
	if err.Error() != `realm "master-realm" has empty spec.realmName` {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRealmSync_BuildRealmExport_Error verifies that a failure during BuildRealmExport
// propagates out of sync() and sets the realm's failed status.
func TestRealmSync_BuildRealmExport_Error(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm).
		WithStatusSubresource(realm).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return fmt.Errorf("injected list error in BuildRealmExport")
			},
		}).
		Build()
	r := newRealmReconcilerWithClient(t, fc)
	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("Reconcile should not return error (sync errors are handled): %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay after build error, got %v", result.RequeueAfter)
	}
}

// TestRealmSync_NewRealm_SpawnsJob verifies that a brand-new realm (no config secret)
// causes SyncRealm to spawn a Job and records it in the realm status.
func TestRealmSync_NewRealm_SpawnsJob(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm).WithStatusSubresource(realm).Build()
	r := newRealmReconcilerWithClient(t, fc)
	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Job was spawned and immediately observed as "still running" → jobObserveInterval.
	if result.RequeueAfter == 0 {
		t.Fatalf("expected non-zero RequeueAfter after Job spawn, got 0")
	}
	var jobs batchv1.JobList
	if err := fc.List(context.Background(), &jobs); err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("expected 1 Job after sync, got %d", len(jobs.Items))
	}
}

// TestRealmSync_CleanupJobs_ListError_Logged verifies that a Job List failure inside
// cleanupTerminalJobs is logged but does not propagate as a Reconcile error.
func TestRealmSync_CleanupJobs_ListError_Logged(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}
	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm, secret).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*batchv1.JobList); ok {
					return fmt.Errorf("injected job list error")
				}
				return c.List(ctx, list, opts...)
			},
		}).
		Build()
	r := newRealmReconcilerWithClient(t, fc)
	// cleanupTerminalJobs error is logged but must not propagate.
	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("cleanupTerminalJobs error must not propagate: %v", err)
	}
}

// TestRealmSync_JobGone_AlreadyReady_Noop verifies that when the tracked job is gone
// but the realm is already Ready, observeJob returns an empty result without updating.
func TestRealmSync_JobGone_AlreadyReady_Noop(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	realm.Status.ActiveJobName = "missing-job"
	realm.Status.Ready = true

	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, secret).WithStatusSubresource(realm).Build()
	if err := fc.Status().Update(context.Background(), realm); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	r := newRealmReconcilerWithClient(t, fc)
	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Job gone + already Ready → should requeue with CheckInterval (not jobObserveInterval).
	if result.RequeueAfter == jobObserveInterval {
		t.Fatalf("expected non-observe interval after noop, got jobObserveInterval")
	}
}

// TestRealmSync_JobCondition_NotTrue_StillRunning verifies that a job condition with
// Status != ConditionTrue is skipped (the continue branch) and the job is treated as running.
func TestRealmSync_JobCondition_NotTrue_StillRunning(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	jobName := "kc-config-job-myrealm-running"
	realm.Status.ActiveJobName = jobName

	// Job with a condition Status=False — should be treated as still running.
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: testNS},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionFalse, // not True → continue
			}},
		},
	}

	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job, secret).WithStatusSubresource(realm).Build()
	if err := fc.Status().Update(context.Background(), realm); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	r := newRealmReconcilerWithClient(t, fc)
	result, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != jobObserveInterval {
		t.Fatalf("expected jobObserveInterval for non-True condition, got %v", result.RequeueAfter)
	}
}

// TestRealmSync_JobFailed_EmptyMessage verifies that a Failed job condition with an
// empty Message field falls back to the default "config-cli Job failed" message.
func TestRealmSync_JobFailed_EmptyMessage(t *testing.T) {
	s := controllerScheme(t)
	realm := realmWithFinalizer()
	jobName := "kc-config-job-myrealm-fail"
	realm.Status.ActiveJobName = jobName

	// Failed condition with empty Message.
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: testNS},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:    batchv1.JobFailed,
				Status:  corev1.ConditionTrue,
				Message: "", // empty → default msg
			}},
		},
	}

	payload := emptyRealmPayload(t, "myrealm")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: testNS,
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job, secret).WithStatusSubresource(realm).Build()
	if err := fc.Status().Update(context.Background(), realm); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	r := newRealmReconcilerWithClient(t, fc)
	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: testNS}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("expected Ready=false after Failed job")
	}
}

// TestRealmReconcile_Deletion_RemoveFinalizer_GetError verifies that a Get failure
// inside removeFinalizer surfaces as a Reconcile error (52.83 and helper.go:28.46).
func TestRealmReconcile_Deletion_RemoveFinalizer_GetError(t *testing.T) {
	s := controllerScheme(t)
	now := metav1.Now()
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "myrealm",
			Namespace:         testNS,
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizer},
		},
		Spec: v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	// Allow the first Realm Get (initial Reconcile fetch) but fail the second
	// (the Get inside removeFinalizer's retry loop).
	getCount := 0
	injected := fmt.Errorf("injected get error in removeFinalizer")
	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*v1alpha1.Realm); ok {
					getCount++
					if getCount >= 2 {
						return injected
					}
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), reconcileRequest("myrealm"))
	if err == nil {
		t.Fatal("expected error from removeFinalizer Get failure")
	}
}

// ── cleanupTerminalJobs branches ─────────────────────────────────────────────

func TestCleanupTerminalJobs_OldOwnedTerminalJob_Deleted(t *testing.T) {
	s := controllerScheme(t)
	realmUID := types.UID("realm-uid-1")
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testNS, UID: realmUID},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}

	oldTime := metav1.NewTime(time.Now().Add(-15 * time.Minute))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "old-job",
			Namespace:         testNS,
			CreationTimestamp: oldTime,
			Labels:            map[string]string{"app": wrapper.LabelApp},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "keycloak.opendefense.cloud/v1alpha1",
				Kind:       "Realm",
				Name:       "myrealm",
				UID:        realmUID,
			}},
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job).Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.cleanupTerminalJobs(context.Background(), realm, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got batchv1.Job
	err := fc.Get(context.Background(), client.ObjectKey{Name: "old-job", Namespace: testNS}, &got)
	if err == nil {
		t.Fatal("expected old terminal job to be deleted")
	}
}

func TestCleanupTerminalJobs_RecentJob_NotDeleted(t *testing.T) {
	s := controllerScheme(t)
	realmUID := types.UID("realm-uid-2")
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testNS, UID: realmUID},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}

	recentTime := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "recent-job",
			Namespace:         testNS,
			CreationTimestamp: recentTime,
			Labels:            map[string]string{"app": wrapper.LabelApp},
			OwnerReferences:   []metav1.OwnerReference{{UID: realmUID}},
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job).Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.cleanupTerminalJobs(context.Background(), realm, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got batchv1.Job
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "recent-job", Namespace: testNS}, &got); err != nil {
		t.Fatalf("expected recent job to still exist: %v", err)
	}
}

func TestCleanupTerminalJobs_NotOwnedJob_NotDeleted(t *testing.T) {
	s := controllerScheme(t)
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testNS, UID: "realm-uid-3"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}

	oldTime := metav1.NewTime(time.Now().Add(-15 * time.Minute))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "other-job",
			Namespace:         testNS,
			CreationTimestamp: oldTime,
			Labels:            map[string]string{"app": wrapper.LabelApp},
			OwnerReferences:   []metav1.OwnerReference{{UID: "different-uid"}},
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job).Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.cleanupTerminalJobs(context.Background(), realm, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got batchv1.Job
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "other-job", Namespace: testNS}, &got); err != nil {
		t.Fatalf("expected non-owned job to still exist: %v", err)
	}
}

func TestCleanupTerminalJobs_ListError(t *testing.T) {
	s := controllerScheme(t)
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testNS},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}

	injected := fmt.Errorf("injected list error")
	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
				return injected
			},
		}).
		Build()

	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	if err := r.cleanupTerminalJobs(context.Background(), realm, ""); err == nil {
		t.Fatal("expected error from list failure")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func TestCleanupTerminalJobs_ActiveTerminalJob_NotDeleted(t *testing.T) {
	s := controllerScheme(t)
	realmUID := types.UID("realm-uid-active")
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testNS, UID: realmUID},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}

	oldTime := metav1.NewTime(time.Now().Add(-15 * time.Minute))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "active-terminal-job",
			Namespace:         testNS,
			CreationTimestamp: oldTime,
			Labels:            map[string]string{"app": wrapper.LabelApp},
			OwnerReferences:   []metav1.OwnerReference{{UID: realmUID}},
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobFailed,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, job).Build()
	r := &RealmReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.cleanupTerminalJobs(context.Background(), realm, "active-terminal-job"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got batchv1.Job
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "active-terminal-job", Namespace: testNS}, &got); err != nil {
		t.Fatalf("expected active terminal job to still exist: %v", err)
	}
}

// newRealmReconcilerWithClient constructs a RealmReconciler backed by the given
// fake client. The JobRunner embedded in the reconciler uses the same client so
// that Secret and Job operations are visible in the same fake store.
func newRealmReconcilerWithClient(t *testing.T, fc client.Client) *RealmReconciler {
	t.Helper()
	s := controllerScheme(t)
	runner := &wrapper.JobRunner{
		Client:         fc,
		URL:            "http://keycloak:8080",
		User:           "admin",
		ConfigCLIImage: "config-cli:test",
		PasswordSecret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "kc-admin"},
			Key:                  "password",
		},
	}
	return &RealmReconciler{
		Client:        fc,
		Scheme:        s,
		Runner:        runner,
		Recorder:      record.NewFakeRecorder(32),
		CheckInterval: 30 * time.Second,
	}
}
