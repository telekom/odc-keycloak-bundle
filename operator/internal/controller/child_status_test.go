package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

// boolPtr is a small helper for optional bool spec fields.
func boolPtr(b bool) *bool { return &b }

// TestClientSync_SetsPendingNotReady verifies F02: after delegating to the Realm
// sync, the Client must NOT be marked Ready (the config-cli Job has not run yet).
func TestClientSync_SetsPendingNotReady(t *testing.T) {
	s := controllerScheme(t)
	ns := "test-ns"

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns},
		Spec: v1alpha1.ClientSpec{
			RealmRef:     "myrealm",
			ClientID:     "app",
			PublicClient: boolPtr(true), // public -> no secret generation
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm, cl).
		WithStatusSubresource(cl).
		Build()

	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}
	if err := r.sync(context.Background(), cl); err != nil {
		t.Fatalf("sync returned error: %v", err)
	}

	var got v1alpha1.Client
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "app", Namespace: ns}, &got); err != nil {
		t.Fatalf("get client: %v", err)
	}
	if got.Status.Ready {
		t.Fatalf("Client must not be Ready before the config-cli Job completes, got Ready=true")
	}
	if got.Status.ObservedGeneration != got.Generation {
		t.Fatalf("ObservedGeneration not updated: %d != %d", got.Status.ObservedGeneration, got.Generation)
	}

	// The Realm must have been annotated to trigger a sync.
	var gotRealm v1alpha1.Realm
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "myrealm", Namespace: ns}, &gotRealm); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if _, ok := gotRealm.Annotations[SyncRequestedAnnotation]; !ok {
		t.Fatalf("expected realm to carry the sync-requested annotation")
	}
}

// TestClientSync_EmptyRealmRefRejected verifies F05: an empty realmRef must be
// rejected by the controller and must never fall back to "master".
func TestClientSync_EmptyRealmRefRejected(t *testing.T) {
	s := controllerScheme(t)
	ns := "test-ns"

	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns},
		Spec:       v1alpha1.ClientSpec{ClientID: "app", PublicClient: boolPtr(true)},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).WithStatusSubresource(cl).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.sync(context.Background(), cl); err == nil {
		t.Fatalf("expected error for empty realmRef, got nil")
	}
}

// TestUserSync_EmptyRealmRefRejected verifies F05 for the User controller.
func TestUserSync_EmptyRealmRefRejected(t *testing.T) {
	s := controllerScheme(t)
	ns := "test-ns"

	u := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns},
		Spec:       v1alpha1.UserSpec{Username: "alice"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(u).WithStatusSubresource(u).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.sync(context.Background(), u); err == nil {
		t.Fatalf("expected error for empty realmRef, got nil")
	}
}

// TestUserSync_SetsPendingNotReady verifies F02 for the User controller.
func TestUserSync_SetsPendingNotReady(t *testing.T) {
	s := controllerScheme(t)
	ns := "test-ns"

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	u := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: ns},
		Spec:       v1alpha1.UserSpec{RealmRef: "myrealm", Username: "alice"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, u).WithStatusSubresource(u).Build()
	r := &UserReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.sync(context.Background(), u); err != nil {
		t.Fatalf("sync returned error: %v", err)
	}
	var got v1alpha1.User
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "alice", Namespace: ns}, &got); err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.Status.Ready {
		t.Fatalf("User must not be Ready before the config-cli Job completes")
	}
}

// TestClientSync_SecretHasOwnerReference verifies F13: the generated client
// secret must carry an OwnerReference to the Client CR for garbage collection.
func TestClientSync_SecretHasOwnerReference(t *testing.T) {
	s := controllerScheme(t)
	ns := "test-ns"

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns},
		Spec: v1alpha1.ClientSpec{
			RealmRef:     "myrealm",
			ClientID:     "confidential-app",
			PublicClient: boolPtr(false), // confidential -> secret generated
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(realm, cl).WithStatusSubresource(cl).Build()
	r := &ClientReconciler{Client: fc, Scheme: s, Recorder: record.NewFakeRecorder(10)}

	if err := r.sync(context.Background(), cl); err != nil {
		t.Fatalf("sync returned error: %v", err)
	}

	var sec corev1.Secret
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "confidential-app-secret", Namespace: ns}, &sec); err != nil {
		t.Fatalf("expected client secret to be created: %v", err)
	}
	if len(sec.OwnerReferences) == 0 {
		t.Fatalf("client secret must have an OwnerReference to the Client CR (F13)")
	}
	found := false
	for _, or := range sec.OwnerReferences {
		if or.Kind == "Client" && or.Name == "app" {
			found = true
		}
	}
	if !found {
		t.Fatalf("client secret OwnerReference does not point to the Client CR: %+v", sec.OwnerReferences)
	}
}
