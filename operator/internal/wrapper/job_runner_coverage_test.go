package wrapper

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

// buildPayload produces the same JSON that SyncRealm would marshal from export.
func buildPayload(t *testing.T, export *RealmExport) []byte {
	t.Helper()
	b, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	return b
}

// TestSyncRealm_IdenticalPayload_RecentSync_Skips verifies that when the config
// Secret already contains the exact same payload and was synced less than 5 min
// ago, SyncRealm returns ("", nil) without creating a new Job.
func TestSyncRealm_IdenticalPayload_RecentSync_Skips(t *testing.T) {
	s := testScheme(t)
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	export := &RealmExport{Realm: "myrealm"}
	payload := buildPayload(t, export)

	// Pre-load config Secret with the same payload and a recent last-sync annotation.
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"last-sync": time.Now().Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(existingSecret).Build()
	jobName, err := testRunner(fc).SyncRealm(context.Background(), realm, export, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jobName != "" {
		t.Fatalf("expected no Job to be created (skip), got job name: %q", jobName)
	}

	// Confirm no Job was created.
	var jobs batchv1.JobList
	if err := fc.List(context.Background(), &jobs); err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 0 {
		t.Fatalf("expected 0 Jobs after skip, got %d", len(jobs.Items))
	}
}

// TestSyncRealm_IdenticalPayload_OldSync_ForcesSync verifies that even with an
// unchanged payload, if more than 5 minutes have elapsed since the last sync,
// a new Job is spawned for drift healing.
func TestSyncRealm_IdenticalPayload_OldSync_ForcesSync(t *testing.T) {
	s := testScheme(t)
	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	export := &RealmExport{Realm: "myrealm"}
	payload := buildPayload(t, export)

	// Last sync was 10 minutes ago — drift healing must trigger.
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-config-myrealm",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"last-sync": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{"realm.json": payload},
	}

	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(existingSecret).Build()
	jobName, err := testRunner(fc).SyncRealm(context.Background(), realm, export, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jobName == "" {
		t.Fatal("expected a Job to be created for drift healing")
	}
}

// TestSyncRealm_EmptyConfigCLIImage_ReturnsError verifies that an unconfigured
// CONFIG_CLI_IMAGE is caught before Job creation and surfaces a clear error.
func TestSyncRealm_EmptyConfigCLIImage_ReturnsError(t *testing.T) {
	s := testScheme(t)
	fc := fake.NewClientBuilder().WithScheme(s).Build()

	runner := &JobRunner{
		Client:         fc,
		URL:            "http://keycloak:8080",
		User:           "admin",
		ConfigCLIImage: "", // intentionally empty
		PasswordSecret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "kc-admin"},
			Key:                  "password",
		},
	}

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	export := &RealmExport{Realm: "myrealm"}

	_, err := runner.SyncRealm(context.Background(), realm, export, s)
	if err == nil {
		t.Fatal("expected error for empty ConfigCLIImage")
	}
	if !containsAny(err.Error(), "CONFIG_CLI_IMAGE", "config-cli image") {
		t.Fatalf("expected error mentioning CONFIG_CLI_IMAGE, got: %v", err)
	}
}

// TestSyncRealm_JobCreateError_Propagates verifies that a failure during Job
// creation is surfaced to the caller rather than silently swallowed.
func TestSyncRealm_JobCreateError_Propagates(t *testing.T) {
	s := testScheme(t)
	injected := fmt.Errorf("injected job create error")

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, ok := obj.(*batchv1.Job); ok {
					return injected
				}
				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	export := &RealmExport{Realm: "myrealm"}

	_, err := testRunner(fc).SyncRealm(context.Background(), realm, export, s)
	if err == nil {
		t.Fatal("expected error from Job create failure")
	}
	if !containsAny(err.Error(), "injected job create error") {
		t.Fatalf("expected injected error message, got: %v", err)
	}
}

// TestSyncRealm_SecretCreateError_Propagates verifies that a failure during
// CreateOrUpdate of the config Secret surfaces as an error rather than creating a Job.
func TestSyncRealm_SecretCreateError_Propagates(t *testing.T) {
	s := testScheme(t)
	injected := fmt.Errorf("injected secret create error")

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, ok := obj.(*corev1.Secret); ok {
					return injected
				}
				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()

	realm := &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
	export := &RealmExport{Realm: "myrealm"}

	_, err := testRunner(fc).SyncRealm(context.Background(), realm, export, s)
	if err == nil {
		t.Fatal("expected error from Secret create failure")
	}
	if !containsAny(err.Error(), "injected secret create error") {
		t.Fatalf("expected injected error in message, got: %v", err)
	}
}

// TestSyncRealm_DiscoverPullSecrets verifies that when POD_NAME is set and the
// operator pod exists in the fake client, imagePullSecrets are propagated to the Job.
func TestSyncRealm_DiscoverPullSecrets(t *testing.T) {
	s := testScheme(t)
	t.Setenv("POD_NAME", "operator-pod")

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "operator-pod", Namespace: "test-ns"},
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "registry-creds"}},
			Containers:       []corev1.Container{{Name: "manager", Image: "operator:latest"}},
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(pod).Build()

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
		t.Fatal("expected a Job to be created")
	}

	var jobs batchv1.JobList
	if err := fc.List(context.Background(), &jobs); err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("expected 1 Job, got %d", len(jobs.Items))
	}
	pullSecrets := jobs.Items[0].Spec.Template.Spec.ImagePullSecrets
	if len(pullSecrets) != 1 || pullSecrets[0].Name != "registry-creds" {
		t.Fatalf("expected imagePullSecrets=[registry-creds] in Job, got: %+v", pullSecrets)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
