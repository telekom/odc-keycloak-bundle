package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClientScopeSpec struct {
	// +kubebuilder:validation:Required
	RealmRef string `json:"realmRef"`
	// +kubebuilder:validation:Required
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol,omitempty"`
	Description string            `json:"description,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type ClientScopeStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kccs
// +kubebuilder:printcolumn:name="Realm",type="string",JSONPath=".spec.realmRef"
// +kubebuilder:printcolumn:name="ScopeName",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="Protocol",type="string",JSONPath=".spec.protocol"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClientScope struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClientScopeSpec   `json:"spec,omitempty"`
	Status            ClientScopeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ClientScopeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientScope `json:"items"`
}

func (in *ClientScope) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(ClientScope)
	in.DeepCopyInto(out)
	return out
}

func (in *ClientScope) DeepCopyInto(out *ClientScope) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.CommonStatus.DeepCopyInto(&out.Status.CommonStatus)
}

func (in *ClientScopeSpec) DeepCopyInto(out *ClientScopeSpec) {
	*out = *in
	if in.Attributes != nil {
		out.Attributes = make(map[string]string, len(in.Attributes))
		for k, v := range in.Attributes {
			out.Attributes[k] = v
		}
	}
}

func (in *ClientScopeList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(ClientScopeList)
	in.DeepCopyInto(out)
	return out
}

func (in *ClientScopeList) DeepCopyInto(out *ClientScopeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]ClientScope, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}
