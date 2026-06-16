package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

// ── IsSafelyDeletedFromRealm ─────────────────────────────────────────────────

func TestIsSafelyDeleted_NilDeletionTimestamp(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).Build()
	safe, err := IsSafelyDeletedFromRealm(context.Background(), fc, "ns", "myrealm", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if safe {
		t.Fatal("expected false for nil deletionTimestamp")
	}
}

func TestIsSafelyDeleted_EmptyRealmRef(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).Build()
	ts := metav1.Now()
	_, err := IsSafelyDeletedFromRealm(context.Background(), fc, "ns", "", &ts)
	if err == nil {
		t.Fatal("expected error for empty realmRef")
	}
}

func TestIsSafelyDeleted_RealmNotFound(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).Build()
	ts := metav1.Now()
	safe, err := IsSafelyDeletedFromRealm(context.Background(), fc, "ns", "gone-realm", &ts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !safe {
		t.Fatal("expected true when realm is not found (cascade delete)")
	}
}

func TestIsSafelyDeleted_RealmReady_LastSyncAfterDeletion(t *testing.T) {
	deletion := metav1.NewTime(time.Now().Add(-2 * time.Minute))
	syncTime := metav1.Now()

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	realm.Status.Ready = true
	realm.Status.LastSyncTime = &syncTime

	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).WithObjects(realm).Build()
	safe, err := IsSafelyDeletedFromRealm(context.Background(), fc, "ns", "myrealm", &deletion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !safe {
		t.Fatal("expected true: realm ready, lastSyncTime after deletion")
	}
}

func TestIsSafelyDeleted_RealmReady_LastSyncBeforeDeletion(t *testing.T) {
	syncTime := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	deletion := metav1.NewTime(time.Now().Add(-2 * time.Minute))

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	realm.Status.Ready = true
	realm.Status.LastSyncTime = &syncTime

	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).WithObjects(realm).Build()
	safe, err := IsSafelyDeletedFromRealm(context.Background(), fc, "ns", "myrealm", &deletion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if safe {
		t.Fatal("expected false: lastSyncTime is before deletion timestamp")
	}
}

func TestIsSafelyDeleted_RealmNotReady(t *testing.T) {
	deletion := metav1.Now()
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	// Status.Ready defaults to false

	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).WithObjects(realm).Build()
	safe, err := IsSafelyDeletedFromRealm(context.Background(), fc, "ns", "myrealm", &deletion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if safe {
		t.Fatal("expected false: realm not ready")
	}
}

func TestIsSafelyDeleted_RealmReady_NilLastSyncTime(t *testing.T) {
	deletion := metav1.Now()
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	realm.Status.Ready = true
	// LastSyncTime intentionally nil

	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).WithObjects(realm).Build()
	safe, err := IsSafelyDeletedFromRealm(context.Background(), fc, "ns", "myrealm", &deletion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if safe {
		t.Fatal("expected false: realm ready but LastSyncTime is nil")
	}
}

// ── TriggerRealmSync ─────────────────────────────────────────────────────────

func TestTriggerRealmSync_EmptyRealmRef(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).Build()
	err := TriggerRealmSync(context.Background(), fc, "ns", "")
	if err == nil {
		t.Fatal("expected error for empty realmRef")
	}
}

func TestTriggerRealmSync_RealmNotFound(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).Build()
	err := TriggerRealmSync(context.Background(), fc, "ns", "missing-realm")
	if err == nil {
		t.Fatal("expected error when realm does not exist")
	}
}

func TestTriggerRealmSync_Success(t *testing.T) {
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	fc := fake.NewClientBuilder().WithScheme(controllerScheme(t)).WithObjects(realm).Build()
	if err := TriggerRealmSync(context.Background(), fc, "ns", "myrealm"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "myrealm", Namespace: "ns"}, &got); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if _, ok := got.Annotations[SyncRequestedAnnotation]; !ok {
		t.Fatal("expected SyncRequestedAnnotation on realm after TriggerRealmSync")
	}
}
