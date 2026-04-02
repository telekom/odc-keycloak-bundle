// +kubebuilder:object:generate=true
// +groupName=keycloak.opendefense.cloud
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "keycloak.opendefense.cloud", Version: "v1alpha1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&Realm{}, &RealmList{},
		&ClientScope{}, &ClientScopeList{},
		&Group{}, &GroupList{},
		&Client{}, &ClientList{},
		&User{}, &UserList{},
		&AuthFlow{}, &AuthFlowList{},
		&IdentityProvider{}, &IdentityProviderList{},
	)
}
