package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RealmSpec struct {
	// +kubebuilder:validation:Required
	RealmName            string `json:"realmName"`
	DisplayName          string `json:"displayName,omitempty"`
	Enabled              *bool  `json:"enabled,omitempty"`
	RegistrationAllowed  *bool  `json:"registrationAllowed,omitempty"`
	ResetPasswordAllowed *bool  `json:"resetPasswordAllowed,omitempty"`
	BruteForceProtected  *bool  `json:"bruteForceProtected,omitempty"`
	LoginTheme           string `json:"loginTheme,omitempty"`
	AccessTokenLifespan  *int   `json:"accessTokenLifespan,omitempty"`
}

type RealmStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kcr
// +kubebuilder:printcolumn:name="RealmName",type="string",JSONPath=".spec.realmName"
// +kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Realm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RealmSpec   `json:"spec,omitempty"`
	Status            RealmStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type RealmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Realm `json:"items"`
}

func (in *Realm) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(Realm)
	in.DeepCopyInto(out)
	return out
}

func (in *Realm) DeepCopyInto(out *Realm) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.CommonStatus.DeepCopyInto(&out.Status.CommonStatus)
}

func (in *RealmSpec) DeepCopyInto(out *RealmSpec) {
	*out = *in
	copyBoolPtr(&in.Enabled, &out.Enabled)
	copyBoolPtr(&in.RegistrationAllowed, &out.RegistrationAllowed)
	copyBoolPtr(&in.ResetPasswordAllowed, &out.ResetPasswordAllowed)
	copyBoolPtr(&in.BruteForceProtected, &out.BruteForceProtected)
	if in.AccessTokenLifespan != nil {
		x := *in.AccessTokenLifespan
		out.AccessTokenLifespan = &x
	}
}

func (in *RealmList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(RealmList)
	in.DeepCopyInto(out)
	return out
}

func (in *RealmList) DeepCopyInto(out *RealmList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Realm, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func copyBoolPtr(src **bool, dst **bool) {
	if *src != nil {
		x := **src
		*dst = &x
	}
}
