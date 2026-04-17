package controller

import (
	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestSetReadyAndFailed(t *testing.T) {
	var s v1alpha1.CommonStatus
	setReady(&s, "kc-123", "synced")
	if !s.Ready || s.KeycloakID != "kc-123" || s.Message != "synced" {
		t.Fatalf("setReady did not set fields correctly: %+v", s)
	}
	if len(s.Conditions) != 1 || s.Conditions[0].Status != metav1.ConditionTrue {
		t.Fatalf("setReady conditions incorrect: %+v", s.Conditions)
	}

	setFailed(&s, "error")
	if s.Ready || s.Message != "error" {
		t.Fatalf("setFailed did not set fields correctly: %+v", s)
	}
	if len(s.Conditions) != 1 || s.Conditions[0].Status != metav1.ConditionFalse {
		t.Fatalf("setFailed conditions incorrect: %+v", s.Conditions)
	}
}
