package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type GroupSpec struct {
	// +kubebuilder:validation:Required
	RealmRef string `json:"realmRef"`
	// +kubebuilder:validation:Required
	Name       string              `json:"name"`
	Path       string              `json:"path,omitempty"`
	Attributes map[string][]string `json:"attributes,omitempty"`
	RealmRoles []string            `json:"realmRoles,omitempty"`
}

type GroupStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kcg
// +kubebuilder:printcolumn:name="Realm",type="string",JSONPath=".spec.realmRef"
// +kubebuilder:printcolumn:name="GroupName",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GroupSpec   `json:"spec,omitempty"`
	Status            GroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group `json:"items"`
}

func (in *Group) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(Group)
	in.DeepCopyInto(out)
	return out
}

func (in *Group) DeepCopyInto(out *Group) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.CommonStatus.DeepCopyInto(&out.Status.CommonStatus)
}

func (in *GroupSpec) DeepCopyInto(out *GroupSpec) {
	*out = *in
	if in.Attributes != nil {
		out.Attributes = make(map[string][]string, len(in.Attributes))
		for k, v := range in.Attributes {
			vals := make([]string, len(v))
			copy(vals, v)
			out.Attributes[k] = vals
		}
	}
	if in.RealmRoles != nil {
		out.RealmRoles = make([]string, len(in.RealmRoles))
		copy(out.RealmRoles, in.RealmRoles)
	}
}

func (in *GroupList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(GroupList)
	in.DeepCopyInto(out)
	return out
}

func (in *GroupList) DeepCopyInto(out *GroupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Group, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}
