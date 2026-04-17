package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// (Removed generic execution tree in favor of Defense Profile Toggles)

type AuthFlowSpec struct {
	// +kubebuilder:validation:Required
	RealmRef string `json:"realmRef,omitempty"`
	// +kubebuilder:validation:Required
	Alias       string `json:"alias"`
	Description string `json:"description,omitempty"`
	TopLevel    bool   `json:"topLevel,omitempty"`

	// Defense Profile Toggles for Authentication flows (e.g. ODC baseline)
	RequireMFA bool `json:"requireMFA,omitempty"`
}

type AuthFlowStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// +kubebuilder:resource:shortName=kcaf
// +kubebuilder:printcolumn:name="Realm",type="string",JSONPath=".spec.realmRef"
// +kubebuilder:printcolumn:name="Alias",type="string",JSONPath=".spec.alias"
// +kubebuilder:printcolumn:name="TopLevel",type="boolean",JSONPath=".spec.topLevel"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type AuthFlow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AuthFlowSpec   `json:"spec,omitempty"`
	Status            AuthFlowStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type AuthFlowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthFlow `json:"items"`
}

// DeepCopy generation

func (in *AuthFlow) DeepCopyInto(out *AuthFlow) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.CommonStatus.DeepCopyInto(&out.Status.CommonStatus)
}

func (in *AuthFlow) DeepCopy() *AuthFlow {
	if in == nil {
		return nil
	}
	out := new(AuthFlow)
	in.DeepCopyInto(out)
	return out
}

func (in *AuthFlow) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *AuthFlowList) DeepCopyInto(out *AuthFlowList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AuthFlow, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *AuthFlowList) DeepCopy() *AuthFlowList {
	if in == nil {
		return nil
	}
	out := new(AuthFlowList)
	in.DeepCopyInto(out)
	return out
}

func (in *AuthFlowList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *AuthFlowSpec) DeepCopyInto(out *AuthFlowSpec) {
	*out = *in
}

func (in *AuthFlowSpec) DeepCopy() *AuthFlowSpec {
	if in == nil {
		return nil
	}
	out := new(AuthFlowSpec)
	in.DeepCopyInto(out)
	return out
}
