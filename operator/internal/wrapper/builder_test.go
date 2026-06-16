package wrapper

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

func testScheme(t *testing.T) *runtime.Scheme {
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

func testRealm(name string) v1alpha1.Realm {
	return v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: name},
	}
}

// TestBuildRealmExport_ListError verifies that a List API failure aborts the
// export and returns an error rather than silently producing an empty slice.
func TestBuildRealmExport_ListError(t *testing.T) {
	s := testScheme(t)
	injected := fmt.Errorf("injected list error")

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
				return injected
			},
		}).
		Build()

	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("test-realm"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if export != nil {
		t.Fatalf("expected nil export on error, got %+v", export)
	}
}

// TestBuildRealmExport_SecretGetError verifies that a missing credential Secret
// surfaces an error rather than being silently skipped.
func TestBuildRealmExport_SecretGetError(t *testing.T) {
	s := testScheme(t)

	// A user that references a password secret which does not exist in the fake client.
	user := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: "test-ns"},
		Spec: v1alpha1.UserSpec{
			RealmRef: "test-realm",
			Username: "alice",
			InitialPassword: &v1alpha1.InitialPasswordRef{
				SecretName: "alice-password",
				SecretKey:  "password",
			},
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(user).
		Build()

	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("test-realm"))
	if err == nil {
		t.Fatal("expected error for missing Secret, got nil")
	}
	if export != nil {
		t.Fatalf("expected nil export on error, got %+v", export)
	}
}

// TestBuildRealmExport_HappyPath verifies that a complete, valid set of objects
// produces a non-nil export with no error.
func TestBuildRealmExport_HappyPath(t *testing.T) {
	s := testScheme(t)

	passwordSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "bob-password", Namespace: "test-ns"},
		Data:       map[string][]byte{"password": []byte("s3cr3t")},
	}
	user := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "bob", Namespace: "test-ns"},
		Spec: v1alpha1.UserSpec{
			RealmRef: "test-realm",
			Username: "bob",
			InitialPassword: &v1alpha1.InitialPasswordRef{
				SecretName: "bob-password",
				SecretKey:  "password",
			},
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(passwordSecret, user).
		Build()

	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("test-realm"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if export == nil {
		t.Fatal("expected non-nil export")
	}
	if len(export.Users) != 1 || export.Users[0].Username != "bob" {
		t.Fatalf("unexpected users in export: %+v", export.Users)
	}
	if len(export.Users[0].Credentials) != 1 || export.Users[0].Credentials[0].Value != "s3cr3t" {
		t.Fatalf("unexpected credentials: %+v", export.Users[0].Credentials)
	}
}

func TestBuildRealmExport_EmptyRealmRefRejected(t *testing.T) {
	s := testScheme(t)
	publicClient := true
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test-ns"},
		Spec: v1alpha1.ClientSpec{
			ClientID:     "app",
			PublicClient: &publicClient,
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(cl).
		Build()

	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("master"))
	if err == nil {
		t.Fatal("expected error for empty realmRef, got nil")
	}
	if export != nil {
		t.Fatalf("expected nil export on error, got %+v", export)
	}
}
