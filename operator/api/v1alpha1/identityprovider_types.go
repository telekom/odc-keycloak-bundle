package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// IdentityProviderSpec defines the desired state of IdentityProvider
type IdentityProviderSpec struct {
	// +kubebuilder:validation:Required
	RealmRef string `json:"realmRef,omitempty"`
	// +kubebuilder:validation:Required
	Alias    string `json:"alias"`
	
	// Type must be strictly "oidc" or "saml"
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=oidc;saml
	Type string `json:"type"`

	Enabled      bool   `json:"enabled,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	StoreToken   bool   `json:"storeToken,omitempty"`
	AddReadTokenRoleOnCreate bool `json:"addReadTokenRoleOnCreate,omitempty"`
	TrustEmail   bool   `json:"trustEmail,omitempty"`
	LinkOnly     bool   `json:"linkOnly,omitempty"`
	FirstBrokerLoginFlowAlias string `json:"firstBrokerLoginFlowAlias,omitempty"`
	PostBrokerLoginFlowAlias  string `json:"postBrokerLoginFlowAlias,omitempty"`

	// Configuration for OIDC or SAML
	Config map[string]string `json:"config,omitempty"`

	// OIDC Client Secret reference
	// +optional
	ClientSecretRef *corev1.SecretKeySelector `json:"clientSecretRef,omitempty"`

	// SAML Signing Certificate reference
	// +optional
	SigningCertificateRef *corev1.SecretKeySelector `json:"signingCertificateRef,omitempty"`
}

// IdentityProviderStatus defines the observed state of IdentityProvider
type IdentityProviderStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// IdentityProvider is the Schema for the IdentityProviders API
// +kubebuilder:resource:shortName=kcip
// +kubebuilder:printcolumn:name="Realm",type="string",JSONPath=".spec.realmRef"
// +kubebuilder:printcolumn:name="Alias",type="string",JSONPath=".spec.alias"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type IdentityProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IdentityProviderSpec   `json:"spec,omitempty"`
	Status IdentityProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IdentityProviderList contains a list of IdentityProvider
type IdentityProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IdentityProvider `json:"items"`
}

func (in *IdentityProvider) DeepCopyInto(out *IdentityProvider) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.CommonStatus.DeepCopyInto(&out.Status.CommonStatus)
}

func (in *IdentityProvider) DeepCopy() *IdentityProvider {
	if in == nil {
		return nil
	}
	out := new(IdentityProvider)
	in.DeepCopyInto(out)
	return out
}

func (in *IdentityProvider) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *IdentityProviderList) DeepCopyInto(out *IdentityProviderList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IdentityProvider, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *IdentityProviderList) DeepCopy() *IdentityProviderList {
	if in == nil {
		return nil
	}
	out := new(IdentityProviderList)
	in.DeepCopyInto(out)
	return out
}

func (in *IdentityProviderList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *IdentityProviderSpec) DeepCopyInto(out *IdentityProviderSpec) {
	*out = *in
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ClientSecretRef != nil {
		in, out := &in.ClientSecretRef, &out.ClientSecretRef
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.SigningCertificateRef != nil {
		in, out := &in.SigningCertificateRef, &out.SigningCertificateRef
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *IdentityProviderSpec) DeepCopy() *IdentityProviderSpec {
	if in == nil {
		return nil
	}
	out := new(IdentityProviderSpec)
	in.DeepCopyInto(out)
	return out
}

