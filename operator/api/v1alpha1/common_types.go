package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// CommonStatus is embedded in all resource status types.
type CommonStatus struct {
	Ready        bool               `json:"ready"`
	KeycloakID   string             `json:"keycloakId,omitempty"`
	Message      string             `json:"message,omitempty"`
	LastSyncTime *metav1.Time       `json:"lastSyncTime,omitempty"`
	Conditions   []metav1.Condition `json:"conditions,omitempty"`
}

func (in *CommonStatus) DeepCopyInto(out *CommonStatus) {
	*out = *in
	if in.LastSyncTime != nil {
		t := *in.LastSyncTime
		out.LastSyncTime = &t
	}
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i, c := range in.Conditions {
			out.Conditions[i] = c
			out.Conditions[i].LastTransitionTime = c.LastTransitionTime
		}
	}
}
