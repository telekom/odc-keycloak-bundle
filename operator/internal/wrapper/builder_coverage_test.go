package wrapper

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

// listErrorOnCall returns an interceptor that injects an error on the nth List call (1-based).
func listErrorOnCall(n int, msg string) interceptor.Funcs {
	count := 0
	return interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			count++
			if count == n {
				return fmt.Errorf("%s", msg)
			}
			return c.List(ctx, list, opts...)
		},
	}
}

// Build order: 1=Client, 2=ClientScope, 3=Group, 4=User, 5=AuthFlow, 6=IdentityProvider

func TestBuildRealmExport_ScopeListError(t *testing.T) {
	fc := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(listErrorOnCall(2, "scope list error")).
		Build()
	_, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err == nil || !strings.Contains(err.Error(), "scope list error") {
		t.Fatalf("expected scope list error, got: %v", err)
	}
}

func TestBuildRealmExport_GroupListError(t *testing.T) {
	fc := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(listErrorOnCall(3, "group list error")).
		Build()
	_, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err == nil || !strings.Contains(err.Error(), "group list error") {
		t.Fatalf("expected group list error, got: %v", err)
	}
}

func TestBuildRealmExport_UserListError(t *testing.T) {
	fc := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(listErrorOnCall(4, "user list error")).
		Build()
	_, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err == nil || !strings.Contains(err.Error(), "user list error") {
		t.Fatalf("expected user list error, got: %v", err)
	}
}

func TestBuildRealmExport_AuthFlowListError(t *testing.T) {
	fc := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(listErrorOnCall(5, "authflow list error")).
		Build()
	_, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err == nil || !strings.Contains(err.Error(), "authflow list error") {
		t.Fatalf("expected authflow list error, got: %v", err)
	}
}

func TestBuildRealmExport_IdentityProviderListError(t *testing.T) {
	fc := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(listErrorOnCall(6, "idp list error")).
		Build()
	_, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err == nil || !strings.Contains(err.Error(), "idp list error") {
		t.Fatalf("expected idp list error, got: %v", err)
	}
}

func TestBuildRealmExport_PublicClient_NoSecretLookup(t *testing.T) {
	s := testScheme(t)
	pub := boolVal(true)
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test-ns"},
		Spec: v1alpha1.ClientSpec{
			RealmRef:     "r",
			ClientID:     "app",
			PublicClient: &pub,
		},
	}
	// No secret in fake client — public client must not attempt a Get.
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(export.Clients) != 1 || export.Clients[0].ClientId != "app" {
		t.Fatalf("expected public client in export, got: %+v", export.Clients)
	}
	if export.Clients[0].Secret != "" {
		t.Fatal("public client must not have a secret in export")
	}
}

func TestBuildRealmExport_UserWithInitialPassword_DefaultKey(t *testing.T) {
	s := testScheme(t)
	pwSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "alice-pw", Namespace: "test-ns"},
		Data:       map[string][]byte{"password": []byte("mypassword")},
	}
	user := &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: "test-ns"},
		Spec: v1alpha1.UserSpec{
			RealmRef: "r",
			Username: "alice",
			InitialPassword: &v1alpha1.InitialPasswordRef{
				SecretName: "alice-pw",
				SecretKey:  "", // empty → defaults to "password"
			},
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(pwSecret, user).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(export.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(export.Users))
	}
	creds := export.Users[0].Credentials
	if len(creds) != 1 || creds[0].Value != "mypassword" {
		t.Fatalf("expected credential with value 'mypassword', got: %+v", creds)
	}
}

func TestBuildRealmExport_IdP_ClientSecretRef_Error(t *testing.T) {
	s := testScheme(t)
	idp := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "ext-oidc", Namespace: "test-ns"},
		Spec: v1alpha1.IdentityProviderSpec{
			RealmRef: "r",
			Alias:    "ext-oidc",
			Type:     "oidc",
			ClientSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "missing-secret"},
				Key:                  "client-secret",
			},
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(idp).Build()
	_, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err == nil {
		t.Fatal("expected error for missing OIDC client secret")
	}
	if !strings.Contains(err.Error(), "OIDC client-secret") {
		t.Fatalf("expected error mentioning 'OIDC client-secret', got: %v", err)
	}
}

func TestBuildRealmExport_IdP_SigningCertRef_Error(t *testing.T) {
	s := testScheme(t)
	idp := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "ext-saml", Namespace: "test-ns"},
		Spec: v1alpha1.IdentityProviderSpec{
			RealmRef: "r",
			Alias:    "ext-saml",
			Type:     "saml",
			SigningCertificateRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "missing-cert"},
				Key:                  "cert",
			},
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(idp).Build()
	_, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err == nil {
		t.Fatal("expected error for missing SAML signing certificate")
	}
	if !strings.Contains(err.Error(), "SAML signing-certificate") {
		t.Fatalf("expected error mentioning 'SAML signing-certificate', got: %v", err)
	}
}

func TestBuildRealmExport_RealmRef_Filtering(t *testing.T) {
	s := testScheme(t)
	clA := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app-a", Namespace: "test-ns"},
		Spec:       v1alpha1.ClientSpec{RealmRef: "realm-a", ClientID: "app-a"},
	}
	clB := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app-b", Namespace: "test-ns"},
		Spec:       v1alpha1.ClientSpec{RealmRef: "realm-b", ClientID: "app-b"},
	}
	// Provide a secret for app-a (confidential by default since PublicClient is nil)
	secretA := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "app-a-secret", Namespace: "test-ns"},
		Data:       map[string][]byte{"CLIENT_SECRET": []byte("shhh")},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(clA, clB, secretA).Build()

	realmA := v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "realm-a", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "realm-a"},
	}
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", realmA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(export.Clients) != 1 || export.Clients[0].ClientId != "app-a" {
		t.Fatalf("expected only realm-a client, got: %+v", export.Clients)
	}
}

func TestBuildRealmExport_SkipsDeleted(t *testing.T) {
	s := testScheme(t)
	now := metav1.NewTime(time.Now())
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "app",
			Namespace:         "test-ns",
			DeletionTimestamp: &now,
			Finalizers:        []string{"test"},
		},
		Spec: v1alpha1.ClientSpec{RealmRef: "r", ClientID: "app"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(export.Clients) != 0 {
		t.Fatalf("expected deleted client to be skipped, got: %+v", export.Clients)
	}
}

func TestBuildRealmExport_AuthFlow_RequireMFA_Adds_OTP_Execution(t *testing.T) {
	s := testScheme(t)
	flow := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser-mfa", Namespace: "test-ns"},
		Spec: v1alpha1.AuthFlowSpec{
			RealmRef:   "r",
			Alias:      "browser-mfa",
			TopLevel:   true,
			RequireMFA: true,
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(flow).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(export.AuthenticationFlows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(export.AuthenticationFlows))
	}
	execs := export.AuthenticationFlows[0].AuthenticationExecutions
	if len(execs) != 2 {
		t.Fatalf("expected 2 executions with MFA, got %d: %+v", len(execs), execs)
	}
	// OTP must be present
	found := false
	for _, e := range execs {
		if e.Authenticator == "auth-otp-form" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected auth-otp-form execution, got: %+v", execs)
	}
}

func TestBuildRealmExport_AuthFlow_NoMFA(t *testing.T) {
	s := testScheme(t)
	flow := &v1alpha1.AuthFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "browser", Namespace: "test-ns"},
		Spec: v1alpha1.AuthFlowSpec{
			RealmRef:   "r",
			Alias:      "browser",
			TopLevel:   true,
			RequireMFA: false,
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(flow).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	execs := export.AuthenticationFlows[0].AuthenticationExecutions
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution without MFA, got %d: %+v", len(execs), execs)
	}
	if execs[0].Authenticator != "auth-username-password-form" {
		t.Fatalf("unexpected authenticator: %s", execs[0].Authenticator)
	}
}

// TestBuildRealmExport_ClientScope_And_Group covers the ClientScope and Group loop
// bodies which are zero in all other tests (all other tests use empty lists).
func TestBuildRealmExport_ClientScope_And_Group(t *testing.T) {
	s := testScheme(t)
	scope := &v1alpha1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: "email", Namespace: "test-ns"},
		Spec: v1alpha1.ClientScopeSpec{
			RealmRef:    "r",
			Name:        "email",
			Protocol:    "openid-connect",
			Description: "email scope",
		},
	}
	group := &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: "test-ns"},
		Spec: v1alpha1.GroupSpec{
			RealmRef: "r",
			Name:     "admins",
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(scope, group).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(export.ClientScopes) != 1 || export.ClientScopes[0].Name != "email" {
		t.Fatalf("expected email scope, got: %+v", export.ClientScopes)
	}
	if len(export.Groups) != 1 || export.Groups[0].Name != "admins" {
		t.Fatalf("expected admins group, got: %+v", export.Groups)
	}
}

// TestBuildRealmExport_IdP_Success covers the full IdP processing path including
// static Config copy, ClientSecretRef lookup success, and SigningCertificateRef lookup success.
func TestBuildRealmExport_IdP_Success(t *testing.T) {
	s := testScheme(t)
	clientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "oidc-secret", Namespace: "test-ns"},
		Data:       map[string][]byte{"client-secret": []byte("super-secret")},
	}
	signingCert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "saml-cert", Namespace: "test-ns"},
		Data:       map[string][]byte{"cert": []byte("-----BEGIN CERT-----")},
	}
	idp := &v1alpha1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "ext-oidc", Namespace: "test-ns"},
		Spec: v1alpha1.IdentityProviderSpec{
			RealmRef: "r",
			Alias:    "ext-oidc",
			Type:     "oidc",
			Config:   map[string]string{"authorizationUrl": "https://idp.example.com/auth"},
			ClientSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "oidc-secret"},
				Key:                  "client-secret",
			},
			SigningCertificateRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "saml-cert"},
				Key:                  "cert",
			},
		},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(idp, clientSecret, signingCert).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", testRealm("r"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(export.IdentityProviders) != 1 {
		t.Fatalf("expected 1 IdP, got %d", len(export.IdentityProviders))
	}
	idpEx := export.IdentityProviders[0]
	if idpEx.Config["authorizationUrl"] != "https://idp.example.com/auth" {
		t.Fatalf("static config not copied: %+v", idpEx.Config)
	}
	if idpEx.Config["clientSecret"] != "super-secret" {
		t.Fatalf("clientSecret not resolved: %+v", idpEx.Config)
	}
	if idpEx.Config["signingCertificate"] != "-----BEGIN CERT-----" {
		t.Fatalf("signingCertificate not resolved: %+v", idpEx.Config)
	}
}

// TestBuildRealmExport_EmptyRealmRefRejectedFromCoverage verifies that an object with
// an empty RealmRef is rejected instead of silently targeting the master realm.
func TestBuildRealmExport_EmptyRealmRefRejectedFromCoverage(t *testing.T) {
	s := testScheme(t)
	pub := true
	cl := &v1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test-ns"},
		Spec:       v1alpha1.ClientSpec{RealmRef: "", ClientID: "app", PublicClient: &pub},
	}
	masterRealm := v1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "master-realm", Namespace: "test-ns"},
		Spec:       v1alpha1.RealmSpec{RealmName: "master"},
	}
	fc := fake.NewClientBuilder().WithScheme(s).WithObjects(cl).Build()
	export, err := BuildRealmExport(context.Background(), fc, "test-ns", masterRealm)
	if err == nil {
		t.Fatal("expected error for empty realmRef, got nil")
	}
	if export != nil {
		t.Fatalf("expected nil export on error, got: %+v", export)
	}
}

func boolVal(b bool) bool { return b }
