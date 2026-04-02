package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClientSpec struct {
	// +kubebuilder:validation:Required
	RealmRef     string   `json:"realmRef,omitempty"`
	// +kubebuilder:validation:Required
	ClientID     string   `json:"clientId"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
	Protocol     string   `json:"protocol,omitempty"`
	PublicClient *bool    `json:"publicClient,omitempty"`
	RedirectUris []string `json:"redirectUris,omitempty"`
	WebOrigins   []string `json:"webOrigins,omitempty"`
}

type ClientStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kcc
// +kubebuilder:printcolumn:name="Realm",type="string",JSONPath=".spec.realmRef"
// +kubebuilder:printcolumn:name="ClientID",type="string",JSONPath=".spec.clientId"
// +kubebuilder:printcolumn:name="Protocol",type="string",JSONPath=".spec.protocol"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Client struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClientSpec   `json:"spec,omitempty"`
	Status            ClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Client `json:"items"`
}

func (in *Client) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(Client)
	in.DeepCopyInto(out)
	return out
}

func (in *Client) DeepCopyInto(out *Client) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.CommonStatus.DeepCopyInto(&out.Status.CommonStatus)
}

func (in *ClientSpec) DeepCopyInto(out *ClientSpec) {
	*out = *in
	copyBoolPtr(&in.Enabled, &out.Enabled)
	copyBoolPtr(&in.PublicClient, &out.PublicClient)
	if in.RedirectUris != nil {
		out.RedirectUris = make([]string, len(in.RedirectUris))
		copy(out.RedirectUris, in.RedirectUris)
	}
	if in.WebOrigins != nil {
		out.WebOrigins = make([]string, len(in.WebOrigins))
		copy(out.WebOrigins, in.WebOrigins)
	}
}

func (in *ClientList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(ClientList)
	in.DeepCopyInto(out)
	return out
}

func (in *ClientList) DeepCopyInto(out *ClientList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Client, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}
