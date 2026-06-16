package controller

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

func controllerScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("add clientgo scheme: %v", err)
	}
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add v1alpha1 scheme: %v", err)
	}
	return s
}

func testRealm() *v1alpha1.Realm {
	return &v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "myrealm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "myrealm"},
	}
}

func testReconciler(c client.Client, s *runtime.Scheme) *RealmReconciler {
	return &RealmReconciler{
		Client:   c,
		Scheme:   s,
		Recorder: record.NewFakeRecorder(10),
	}
}

func TestRealmDeepCopy_PreservesActiveJobName(t *testing.T) {
	realm := testRealm()
	realm.Status.ActiveJobName = "kc-config-job-myrealm-abc"

	copied := realm.DeepCopy()
	if copied.Status.ActiveJobName != realm.Status.ActiveJobName {
		t.Fatalf("ActiveJobName lost during DeepCopy: %q", copied.Status.ActiveJobName)
	}
}

// TestObserveJob_Running verifies that a running Job results in Ready=false and a short requeue.
func TestObserveJob_Running(t *testing.T) {
	s := controllerScheme(t)
	ns, realmName := "test-ns", "myrealm"

	realm := testRealm()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "kc-config-job-myrealm-abc", Namespace: ns},
		// No conditions — still running.
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm, job).
		WithStatusSubresource(realm).
		Build()

	r := testReconciler(fc, s)
	result, err := r.observeJob(context.Background(), realm, job.Name, realmName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != jobObserveInterval {
		t.Fatalf("expected requeue after %v, got %v", jobObserveInterval, result.RequeueAfter)
	}

	var updated v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: realmName}, &updated); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if updated.Status.Ready {
		t.Fatal("expected Ready=false while Job is running")
	}
	if updated.Status.Message != "Config-CLI Job running" {
		t.Fatalf("unexpected message: %q", updated.Status.Message)
	}
}

// TestObserveJob_Succeeded verifies that a completed Job sets Ready=true.
func TestObserveJob_Succeeded(t *testing.T) {
	s := controllerScheme(t)
	ns, realmName := "test-ns", "myrealm"

	realm := testRealm()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "kc-config-job-myrealm-abc", Namespace: ns},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm, job).
		WithStatusSubresource(realm).
		Build()

	r := testReconciler(fc, s)
	result, err := r.observeJob(context.Background(), realm, job.Name, realmName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no fast-requeue after success, got %v", result.RequeueAfter)
	}

	var updated v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: realmName}, &updated); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if !updated.Status.Ready {
		t.Fatal("expected Ready=true after Job succeeded")
	}
}

// TestObserveJob_Failed verifies that a failed Job sets Ready=false with the failure message.
func TestObserveJob_Failed(t *testing.T) {
	s := controllerScheme(t)
	ns, realmName := "test-ns", "myrealm"

	realm := testRealm()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "kc-config-job-myrealm-abc", Namespace: ns},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:    batchv1.JobFailed,
				Status:  corev1.ConditionTrue,
				Message: "BackoffLimitExceeded",
			}},
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm, job).
		WithStatusSubresource(realm).
		Build()

	r := testReconciler(fc, s)
	result, err := r.observeJob(context.Background(), realm, job.Name, realmName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no fast-requeue after failure, got %v", result.RequeueAfter)
	}

	var updated v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: realmName}, &updated); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if updated.Status.Ready {
		t.Fatal("expected Ready=false after Job failed")
	}
	if updated.Status.Message != "BackoffLimitExceeded" {
		t.Fatalf("unexpected message: %q", updated.Status.Message)
	}
}

// TestObserveJob_NotFound verifies that a missing active Job does not create a false Ready status.
func TestObserveJob_NotFound(t *testing.T) {
	s := controllerScheme(t)
	ns, realmName := "test-ns", "myrealm"

	realm := testRealm()

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(realm).
		WithStatusSubresource(realm).
		Build()

	r := testReconciler(fc, s)
	result, err := r.observeJob(context.Background(), realm, "kc-config-job-myrealm-gone", realmName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no fast-requeue after not-found, got %v", result.RequeueAfter)
	}

	var updated v1alpha1.Realm
	if err := fc.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: realmName}, &updated); err != nil {
		t.Fatalf("get realm: %v", err)
	}
	if updated.Status.Ready {
		t.Fatal("expected Ready=false when active Job is missing")
	}
	if updated.Status.Message != "active config-cli Job disappeared before a terminal result was observed" {
		t.Fatalf("unexpected message: %q", updated.Status.Message)
	}
}
