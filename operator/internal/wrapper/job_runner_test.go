package wrapper

import (
	"context"
	"fmt"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

func testRunner(c client.Client) *JobRunner {
	return &JobRunner{
		Client: c,
		URL:    "http://keycloak:8080",
		User:   "admin",
		PasswordSecret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "kc-admin"},
			Key:                  "password",
		},
		ConfigCLIImage: "config-cli:latest",
	}
}

// TestSyncRealm_SecretGetError verifies that a non-NotFound GET error on the
// config Secret is propagated so the reconciler retries rather than silently skipping.
func TestSyncRealm_SecretGetError(t *testing.T) {
	s := testScheme(t)

	injected := fmt.Errorf("injected server error")
	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*corev1.Secret); ok {
					return injected
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "test-realm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "test-realm"},
	}
	export := &RealmExport{Realm: "test-realm"}

	_, err := testRunner(fc).SyncRealm(context.Background(), realm, export, s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "injected server error") {
		t.Fatalf("expected error to mention injected message, got: %v", err)
	}
}

// TestSyncRealm_CreateSecret verifies that SyncRealm creates the config Secret
// when it does not yet exist and launches a Job.
func TestSyncRealm_CreateSecret(t *testing.T) {
	s := testScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	export := &RealmExport{Realm: "myrealm"}

	jobName, err := testRunner(fc).SyncRealm(context.Background(), realm, export, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jobName == "" {
		t.Fatal("expected non-empty job name")
	}

	var secret corev1.Secret
	if err := fc.Get(context.Background(), client.ObjectKey{Namespace: "test-ns", Name: "kc-config-myrealm"}, &secret); err != nil {
		t.Fatalf("expected config Secret to exist: %v", err)
	}
	if _, ok := secret.Data["realm.json"]; !ok {
		t.Fatal("expected realm.json key in secret data")
	}
}

// TestSyncRealm_NoDeleteRace verifies that SyncRealm no longer deletes an existing
// Job before creating a new one. The old Job must still exist after the call.
func TestSyncRealm_NoDeleteRace(t *testing.T) {
	s := testScheme(t)

	ns := "test-ns"
	realmName := "myrealm"
	oldJobName := fmt.Sprintf("kc-config-job-%s-abc123", realmName)

	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oldJobName,
			Namespace: ns,
			Labels:    map[string]string{"app": LabelApp},
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(existingJob).
		Build()

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: realmName, Namespace: ns},
		Spec:       v1alpha1.RealmSpec{RealmName: realmName},
	}
	export := &RealmExport{Realm: realmName}

	newJobName, err := testRunner(fc).SyncRealm(context.Background(), realm, export, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newJobName == "" {
		t.Fatal("expected non-empty job name to be returned")
	}

	// Old job must still exist — SyncRealm must not delete it.
	var oldJob batchv1.Job
	if err := fc.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: oldJobName}, &oldJob); err != nil {
		t.Fatalf("old job was deleted by SyncRealm: %v", err)
	}

	// New job must have a different name (GenerateName produced a unique suffix).
	if newJobName == oldJobName {
		t.Fatalf("new job name %q collides with old job name", newJobName)
	}
	var newJob batchv1.Job
	if err := fc.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: newJobName}, &newJob); err != nil {
		t.Fatalf("new job %q not found: %v", newJobName, err)
	}
}
