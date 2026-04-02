package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserSpec struct {
	// +kubebuilder:validation:Required
	RealmRef        string                  `json:"realmRef"`
	// +kubebuilder:validation:Required
	Username        string                  `json:"username"`
	Email           string                  `json:"email,omitempty"`
	FirstName       string                  `json:"firstName,omitempty"`
	LastName        string                  `json:"lastName,omitempty"`
	Enabled         *bool                   `json:"enabled,omitempty"`
	EmailVerified   *bool                   `json:"emailVerified,omitempty"`
	Groups          []string                `json:"groups,omitempty"`
	InitialPassword *InitialPasswordRef     `json:"initialPassword,omitempty"`
}

type InitialPasswordRef struct {
	SecretName string `json:"secretName"`
	SecretKey  string `json:"secretKey,omitempty"`
}

type UserStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kcu
// +kubebuilder:printcolumn:name="Realm",type="string",JSONPath=".spec.realmRef"
// +kubebuilder:printcolumn:name="Username",type="string",JSONPath=".spec.username"
// +kubebuilder:printcolumn:name="Email",type="string",JSONPath=".spec.email"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserSpec   `json:"spec,omitempty"`
	Status            UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func (in *User) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(User)
	in.DeepCopyInto(out)
	return out
}

func (in *User) DeepCopyInto(out *User) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.CommonStatus.DeepCopyInto(&out.Status.CommonStatus)
}

func (in *UserSpec) DeepCopyInto(out *UserSpec) {
	*out = *in
	copyBoolPtr(&in.Enabled, &out.Enabled)
	copyBoolPtr(&in.EmailVerified, &out.EmailVerified)
	if in.Groups != nil {
		out.Groups = make([]string, len(in.Groups))
		copy(out.Groups, in.Groups)
	}
	if in.InitialPassword != nil {
		pw := *in.InitialPassword
		out.InitialPassword = &pw
	}
}

func (in *UserList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(UserList)
	in.DeepCopyInto(out)
	return out
}

func (in *UserList) DeepCopyInto(out *UserList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]User, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}
