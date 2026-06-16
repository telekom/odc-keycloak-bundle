package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

// ── shared helpers ────────────────────────────────────────────────────────────

const testChildNS = "ns"

// safeRealm returns a Realm in testChildNS with Ready=true and a LastSyncTime safely in the
// future relative to any deletion timestamp the test will use.
func safeRealm() *v1alpha1.Realm {
	now := metav1.Now()
	r := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testChildNS},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	r.Status.Ready = true
	r.Status.LastSyncTime = &now
	return r
}

// notReadyRealm returns a Realm in testChildNS with Ready=false.
func notReadyRealm() *v1alpha1.Realm {
	return &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: testChildNS},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
}

// pastTime returns a timestamp 2 minutes in the past, useful for DeletionTimestamp
// so that safeRealm's LastSyncTime (set to Now) is after the deletion.
func pastTime() *metav1.Time {
	t := metav1.NewTime(time.Now().Add(-2 * time.Minute))
	return &t
}

// req builds a ctrl.Request for name in testChildNS.
func req(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: testChildNS}}
}

// realmGetErrorFuncs returns interceptor.Funcs that injects err when a Realm is Get'd.
func realmGetErrorFuncs(err error) interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*v1alpha1.Realm); ok {
				return err
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}
}

// ── ClientReconciler ──────────────────────────────────────────────────────────

func TestClientReconcile_NotFound(t *testing.T) {
	s := controllerScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	result, err := r.Reconcile(context.Background(), req("missing"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestClientReconcile_AddFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns},
		Spec:       v1alpha1.ClientSpec{RealmRef: "myrealm", ClientID: "app"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	_, err := r.Reconcile(context.Background(), req("app"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.Client
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "app", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	found := false
	for _, f := range got.Finalizers {
		if f == finalizer {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected finalizer, got %v", got.Finalizers)
	}
}

func TestClientReconcile_Deletion_NoFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	// External finalizer keeps the object alive in the fake tracker after Delete().
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Finalizers: []string{"kubernetes.io/pvc-protection"}},
		Spec:       v1alpha1.ClientSpec{RealmRef: "myrealm", ClientID: "app"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).Build()
	if err := fc.Delete(context.Background(), cl); err != nil {
		t.Fatalf("delete: %v", err)
	}
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	result, err := r.Reconcile(context.Background(), req("app"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestClientReconcile_Deletion_NotSafe_Requeues(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app", Namespace: ns,
			DeletionTimestamp: pastTime(),
			Finalizers:        []string{finalizer},
		},
		Spec: v1alpha1.ClientSpec{RealmRef: "myrealm", ClientID: "app"},
	}
	// Realm exists but is not ready → deletion not safe yet.
	realm := notReadyRealm()
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl, realm).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	result, err := r.Reconcile(context.Background(), req("app"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 5*time.Second {
		t.Fatalf("expected 5s requeue, got %v", result.RequeueAfter)
	}
}

func TestClientReconcile_Deletion_Safe_RemovesFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app", Namespace: ns,
			DeletionTimestamp: pastTime(),
			Finalizers:        []string{finalizer},
		},
		Spec: v1alpha1.ClientSpec{RealmRef: "myrealm", ClientID: "app"},
	}
	realm := safeRealm()
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl, realm).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	_, err := r.Reconcile(context.Background(), req("app"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Object should be deleted or finalizer removed.
	var got v1alpha1.Client
	getErr := fc.Get(context.Background(), client.ObjectKey{Name: "app", Namespace: ns}, &got)
	if getErr == nil {
		for _, f := range got.Finalizers {
			if f == finalizer {
				t.Fatalf("finalizer still present: %v", got.Finalizers)
			}
		}
	}
}

func TestClientReconcile_Deletion_IsSafelyDeleted_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientSpec{RealmRef: "myrealm", ClientID: "app"},
	}
	injected := fmt.Errorf("injected realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("app"))
	if err == nil {
		t.Fatal("expected error from IsSafelyDeletedFromRealm Get failure")
	}
}

func TestClientReconcile_SyncError_SetsFailedStatus(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	// RealmRef="" triggers sync error path.
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientSpec{ClientID: "app"}, // RealmRef intentionally empty
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).WithStatusSubresource(cl).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	result, err := r.Reconcile(context.Background(), req("app"))
	if err != nil {
		t.Fatalf("unexpected error from Reconcile: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
	var got v1alpha1.Client
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "app", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("expected Ready=false after sync error")
	}
}

func TestClientReconcile_SyncSuccess_SetsPending(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Finalizers: []string{finalizer}},
		Spec: v1alpha1.ClientSpec{
			RealmRef:     "myrealm",
			ClientID:     "app",
			PublicClient: boolPtr(true),
		},
	}
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl, realm).WithStatusSubresource(cl).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10), CheckInterval: 30 * time.Second}

	result, err := r.Reconcile(context.Background(), req("app"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected CheckInterval, got %v", result.RequeueAfter)
	}
	var got v1alpha1.Client
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "app", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("Client must be Pending (not Ready) before config-cli Job completes")
	}
}

func TestClientSync_TriggerRealmSync_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientSpec{RealmRef: "myrealm", ClientID: "app", PublicClient: boolPtr(true)},
	}
	injected := fmt.Errorf("injected non-notfound realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).WithStatusSubresource(cl).
		WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("app"))
	if err != nil {
		t.Fatalf("Reconcile must not propagate sync errors: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

// ── GroupReconciler ───────────────────────────────────────────────────────────

func TestGroupReconcile_NotFound(t *testing.T) {
	s := controllerScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("missing"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestGroupReconcile_AddFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns},
		Spec:       v1alpha1.GroupSpec{RealmRef: "myrealm", Name: "admins"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	if _, err := r.Reconcile(context.Background(), req("admins")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.Group
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "admins", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, f := range got.Finalizers {
		if f == finalizer {
			return
		}
	}
	t.Fatalf("expected finalizer, got %v", got.Finalizers)
}

func TestGroupReconcile_Deletion_NoFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns, Finalizers: []string{"kubernetes.io/pvc-protection"}},
		Spec:       v1alpha1.GroupSpec{RealmRef: "myrealm", Name: "admins"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	if err := fc.Delete(context.Background(), obj); err != nil {
		t.Fatalf("delete: %v", err)
	}
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("admins"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestGroupReconcile_Deletion_NotSafe_Requeues(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.GroupSpec{RealmRef: "myrealm", Name: "admins"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, notReadyRealm()).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("admins"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 5*time.Second {
		t.Fatalf("expected 5s requeue, got %v", result.RequeueAfter)
	}
}

func TestGroupReconcile_Deletion_Safe_RemovesFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.GroupSpec{RealmRef: "myrealm", Name: "admins"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, safeRealm()).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("admins"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.Group
	getErr := fc.Get(context.Background(), client.ObjectKey{Name: "admins", Namespace: ns}, &got)
	if getErr == nil {
		for _, f := range got.Finalizers {
			if f == finalizer {
				t.Fatalf("finalizer still present: %v", got.Finalizers)
			}
		}
	}
}

func TestGroupReconcile_Deletion_IsSafelyDeleted_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.GroupSpec{RealmRef: "myrealm", Name: "admins"},
	}
	injected := fmt.Errorf("injected realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("admins"))
	if err == nil {
		t.Fatal("expected error from IsSafelyDeletedFromRealm Get failure")
	}
}

func TestGroupSync_EmptyRealmRef(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.GroupSpec{Name: "admins"}, // RealmRef empty
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("admins"))
	if err != nil {
		t.Fatalf("unexpected error from Reconcile: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

func TestGroupSync_SetsPending(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.GroupSpec{RealmRef: "myrealm", Name: "admins"},
	}
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, realm).WithStatusSubresource(obj).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10), CheckInterval: 30 * time.Second}
	if _, err := r.Reconcile(context.Background(), req("admins")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.Group
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "admins", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Ready {
		t.Fatal("Group must be Pending before config-cli Job completes")
	}
}

func TestGroupSync_TriggerRealmSync_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.GroupSpec{RealmRef: "myrealm", Name: "admins"},
	}
	injected := fmt.Errorf("injected non-notfound realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).
		WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &GroupReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("admins"))
	if err != nil {
		t.Fatalf("Reconcile must not propagate sync errors: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay after TriggerRealmSync error, got %v", result.RequeueAfter)
	}
}

// ── ClientScopeReconciler ─────────────────────────────────────────────────────

func TestClientScopeReconcile_NotFound(t *testing.T) {
	s := controllerScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("missing"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestClientScopeReconcile_AddFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns},
		Spec:       v1alpha1.ClientScopeSpec{RealmRef: "myrealm", Name: "email"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	if _, err := r.Reconcile(context.Background(), req("email")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.ClientScope
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "email", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, f := range got.Finalizers {
		if f == finalizer {
			return
		}
	}
	t.Fatalf("expected finalizer, got %v", got.Finalizers)
}

func TestClientScopeReconcile_Deletion_NoFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns, Finalizers: []string{"kubernetes.io/pvc-protection"}},
		Spec:       v1alpha1.ClientScopeSpec{RealmRef: "myrealm", Name: "email"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	if err := fc.Delete(context.Background(), obj); err != nil {
		t.Fatalf("delete: %v", err)
	}
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("email"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestClientScopeReconcile_Deletion_NotSafe_Requeues(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientScopeSpec{RealmRef: "myrealm", Name: "email"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, notReadyRealm()).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("email"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 5*time.Second {
		t.Fatalf("expected 5s requeue, got %v", result.RequeueAfter)
	}
}

func TestClientScopeReconcile_Deletion_Safe_RemovesFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientScopeSpec{RealmRef: "myrealm", Name: "email"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, safeRealm()).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("email"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.ClientScope
	getErr := fc.Get(context.Background(), client.ObjectKey{Name: "email", Namespace: ns}, &got)
	if getErr == nil {
		for _, f := range got.Finalizers {
			if f == finalizer {
				t.Fatalf("finalizer still present: %v", got.Finalizers)
			}
		}
	}
}

func TestClientScopeReconcile_Deletion_IsSafelyDeleted_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientScopeSpec{RealmRef: "myrealm", Name: "email"},
	}
	injected := fmt.Errorf("injected realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("email"))
	if err == nil {
		t.Fatal("expected error from IsSafelyDeletedFromRealm Get failure")
	}
}

func TestClientScopeSync_EmptyRealmRef(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientScopeSpec{Name: "email"}, // RealmRef empty
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("email"))
	if err != nil {
		t.Fatalf("unexpected error from Reconcile: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

func TestClientScopeSync_SetsPending(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientScopeSpec{RealmRef: "myrealm", Name: "email"},
	}
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, realm).WithStatusSubresource(obj).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10), CheckInterval: 30 * time.Second}
	result, err := r.Reconcile(context.Background(), req("email"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected CheckInterval, got %v", result.RequeueAfter)
	}
}

func TestClientScopeSync_TriggerRealmSync_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.ClientScopeSpec{RealmRef: "myrealm", Name: "email"},
	}
	injected := fmt.Errorf("injected non-notfound realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).
		WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &ClientScopeReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("email"))
	if err != nil {
		t.Fatalf("Reconcile must not propagate sync errors: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

// ── AuthFlowReconciler ────────────────────────────────────────────────────────

func TestAuthFlowReconcile_NotFound(t *testing.T) {
	s := controllerScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("missing"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestAuthFlowReconcile_AddFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns},
		Spec:       v1alpha1.AuthFlowSpec{RealmRef: "myrealm", Alias: "browser"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	if _, err := r.Reconcile(context.Background(), req("browser")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.AuthFlow
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "browser", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, f := range got.Finalizers {
		if f == finalizer {
			return
		}
	}
	t.Fatalf("expected finalizer, got %v", got.Finalizers)
}

func TestAuthFlowReconcile_Deletion_NoFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns, Finalizers: []string{"kubernetes.io/pvc-protection"}},
		Spec:       v1alpha1.AuthFlowSpec{RealmRef: "myrealm", Alias: "browser"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	if err := fc.Delete(context.Background(), obj); err != nil {
		t.Fatalf("delete: %v", err)
	}
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("browser"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestAuthFlowReconcile_Deletion_NotSafe_Requeues(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.AuthFlowSpec{RealmRef: "myrealm", Alias: "browser"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, notReadyRealm()).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("browser"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 5*time.Second {
		t.Fatalf("expected 5s requeue, got %v", result.RequeueAfter)
	}
}

func TestAuthFlowReconcile_Deletion_Safe_RemovesFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.AuthFlowSpec{RealmRef: "myrealm", Alias: "browser"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, safeRealm()).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("browser"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.AuthFlow
	getErr := fc.Get(context.Background(), client.ObjectKey{Name: "browser", Namespace: ns}, &got)
	if getErr == nil {
		for _, f := range got.Finalizers {
			if f == finalizer {
				t.Fatalf("finalizer still present: %v", got.Finalizers)
			}
		}
	}
}

func TestAuthFlowReconcile_Deletion_IsSafelyDeleted_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.AuthFlowSpec{RealmRef: "myrealm", Alias: "browser"},
	}
	injected := fmt.Errorf("injected realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("browser"))
	if err == nil {
		t.Fatal("expected error from IsSafelyDeletedFromRealm Get failure")
	}
}

func TestAuthFlowSync_EmptyRealmRef(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.AuthFlowSpec{Alias: "browser"}, // RealmRef empty
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("browser"))
	if err != nil {
		t.Fatalf("unexpected error from Reconcile: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

func TestAuthFlowSync_SetsPending(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.AuthFlowSpec{RealmRef: "myrealm", Alias: "browser"},
	}
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, realm).WithStatusSubresource(obj).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10), CheckInterval: 30 * time.Second}
	result, err := r.Reconcile(context.Background(), req("browser"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected CheckInterval, got %v", result.RequeueAfter)
	}
}

func TestAuthFlowSync_TriggerRealmSync_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.AuthFlowSpec{RealmRef: "myrealm", Alias: "browser"},
	}
	injected := fmt.Errorf("injected non-notfound realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).
		WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &AuthFlowReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("browser"))
	if err != nil {
		t.Fatalf("Reconcile must not propagate sync errors: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

// ── IdentityProviderReconciler ────────────────────────────────────────────────

func TestIdentityProviderReconcile_NotFound(t *testing.T) {
	s := controllerScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("missing"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestIdentityProviderReconcile_AddFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns},
		Spec:       v1alpha1.IdentityProviderSpec{RealmRef: "myrealm", Alias: "google", Type: "oidc"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	if _, err := r.Reconcile(context.Background(), req("google")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.IdentityProvider
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "google", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, f := range got.Finalizers {
		if f == finalizer {
			return
		}
	}
	t.Fatalf("expected finalizer, got %v", got.Finalizers)
}

func TestIdentityProviderReconcile_Deletion_NoFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns, Finalizers: []string{"kubernetes.io/pvc-protection"}},
		Spec:       v1alpha1.IdentityProviderSpec{RealmRef: "myrealm", Alias: "google", Type: "oidc"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	if err := fc.Delete(context.Background(), obj); err != nil {
		t.Fatalf("delete: %v", err)
	}
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("google"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestIdentityProviderReconcile_Deletion_NotSafe_Requeues(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.IdentityProviderSpec{RealmRef: "myrealm", Alias: "google", Type: "oidc"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, notReadyRealm()).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("google"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 5*time.Second {
		t.Fatalf("expected 5s requeue, got %v", result.RequeueAfter)
	}
}

func TestIdentityProviderReconcile_Deletion_Safe_RemovesFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.IdentityProviderSpec{RealmRef: "myrealm", Alias: "google", Type: "oidc"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, safeRealm()).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("google"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.IdentityProvider
	getErr := fc.Get(context.Background(), client.ObjectKey{Name: "google", Namespace: ns}, &got)
	if getErr == nil {
		for _, f := range got.Finalizers {
			if f == finalizer {
				t.Fatalf("finalizer still present: %v", got.Finalizers)
			}
		}
	}
}

func TestIdentityProviderReconcile_Deletion_IsSafelyDeleted_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.IdentityProviderSpec{RealmRef: "myrealm", Alias: "google", Type: "oidc"},
	}
	injected := fmt.Errorf("injected realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("google"))
	if err == nil {
		t.Fatal("expected error from IsSafelyDeletedFromRealm Get failure")
	}
}

func TestIdentityProviderSync_EmptyRealmRef(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.IdentityProviderSpec{Alias: "google", Type: "oidc"}, // RealmRef empty
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("google"))
	if err != nil {
		t.Fatalf("unexpected error from Reconcile: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

func TestIdentityProviderSync_SetsPending(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.IdentityProviderSpec{RealmRef: "myrealm", Alias: "google", Type: "oidc"},
	}
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, realm).WithStatusSubresource(obj).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10), CheckInterval: 30 * time.Second}
	result, err := r.Reconcile(context.Background(), req("google"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected CheckInterval, got %v", result.RequeueAfter)
	}
}

func TestIdentityProviderSync_TriggerRealmSync_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "google", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.IdentityProviderSpec{RealmRef: "myrealm", Alias: "google", Type: "oidc"},
	}
	injected := fmt.Errorf("injected non-notfound realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).
		WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &IdentityProviderReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("google"))
	if err != nil {
		t.Fatalf("Reconcile must not propagate sync errors: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

// ── UserReconciler — Reconcile() paths ───────────────────────────────────────
// (sync() paths already covered by child_status_test.go)

func TestUserReconcile_NotFound(t *testing.T) {
	s := controllerScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("missing"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestUserReconcile_AddFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns},
		Spec:       v1alpha1.UserSpec{RealmRef: "myrealm", Username: "alice"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	if _, err := r.Reconcile(context.Background(), req("alice")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.User
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "alice", Namespace: ns}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, f := range got.Finalizers {
		if f == finalizer {
			return
		}
	}
	t.Fatalf("expected finalizer, got %v", got.Finalizers)
}

func TestUserReconcile_Deletion_NoFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns, Finalizers: []string{"kubernetes.io/pvc-protection"}},
		Spec:       v1alpha1.UserSpec{RealmRef: "myrealm", Username: "alice"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
	if err := fc.Delete(context.Background(), obj); err != nil {
		t.Fatalf("delete: %v", err)
	}
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("alice"))
	if err != nil || result.RequeueAfter != 0 {
		t.Fatalf("want ({}, nil), got (%v, %v)", result, err)
	}
}

func TestUserReconcile_Deletion_NotSafe_Requeues(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.UserSpec{RealmRef: "myrealm", Username: "alice"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, notReadyRealm()).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("alice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 5*time.Second {
		t.Fatalf("expected 5s requeue, got %v", result.RequeueAfter)
	}
}

func TestUserReconcile_Deletion_Safe_RemovesFinalizer(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.UserSpec{RealmRef: "myrealm", Username: "alice"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj, safeRealm()).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("alice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got v1alpha1.User
	getErr := fc.Get(context.Background(), client.ObjectKey{Name: "alice", Namespace: ns}, &got)
	if getErr == nil {
		for _, f := range got.Finalizers {
			if f == finalizer {
				t.Fatalf("finalizer still present: %v", got.Finalizers)
			}
		}
	}
}

func TestUserReconcile_Deletion_IsSafelyDeleted_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns, DeletionTimestamp: pastTime(), Finalizers: []string{finalizer}},
		Spec:       v1alpha1.UserSpec{RealmRef: "myrealm", Username: "alice"},
	}
	injected := fmt.Errorf("injected realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	_, err := r.Reconcile(context.Background(), req("alice"))
	if err == nil {
		t.Fatal("expected error from IsSafelyDeletedFromRealm Get failure")
	}
}

func TestUserSync_TriggerRealmSync_GetError(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.UserSpec{RealmRef: "myrealm", Username: "alice"},
	}
	injected := fmt.Errorf("injected non-notfound realm get error")
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).
		WithInterceptorFuncs(realmGetErrorFuncs(injected)).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("alice"))
	if err != nil {
		t.Fatalf("Reconcile must not propagate sync errors: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}

func TestUserReconcile_SyncError_SetsFailedStatus(t *testing.T) {
	s := controllerScheme(t)
	ns := "ns"
	obj := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns, Finalizers: []string{finalizer}},
		Spec:       v1alpha1.UserSpec{Username: "alice"}, // RealmRef empty → sync error
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).WithStatusSubresource(obj).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	result, err := r.Reconcile(context.Background(), req("alice"))
	if err != nil {
		t.Fatalf("unexpected error from Reconcile: %v", err)
	}
	if result.RequeueAfter != requeueDelay {
		t.Fatalf("expected requeueDelay, got %v", result.RequeueAfter)
	}
}
