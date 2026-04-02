package wrapper

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/opendefensecloud/keycloak-bundle/operator/api/v1alpha1"
)

type RealmExport struct {
	Realm                string                `json:"realm"`
	DisplayName          string                `json:"displayName,omitempty"`
	Enabled              *bool                 `json:"enabled,omitempty"`
	RegistrationAllowed  *bool                 `json:"registrationAllowed,omitempty"`
	ResetPasswordAllowed *bool                 `json:"resetPasswordAllowed,omitempty"`
	BruteForceProtected  *bool                 `json:"bruteForceProtected,omitempty"`
	LoginTheme           string                `json:"loginTheme,omitempty"`
	AccessTokenLifespan  *int                  `json:"accessTokenLifespan,omitempty"`
	Clients              []ClientExport        `json:"clients"`
	Users                []UserExport          `json:"users"`
	Groups               []GroupExport         `json:"groups"`
	ClientScopes         []ClientScopeExport   `json:"clientScopes"`
	AuthenticationFlows  []AuthFlowExport      `json:"authenticationFlows"`
	IdentityProviders    []IdentityProviderExport `json:"identityProviders"`
}

type ClientExport struct {
	ClientId     string   `json:"clientId"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
	Protocol     string   `json:"protocol,omitempty"`
	PublicClient *bool    `json:"publicClient,omitempty"`
	Secret       string   `json:"secret,omitempty"`
	RedirectUris []string `json:"redirectUris,omitempty"`
	WebOrigins   []string `json:"webOrigins,omitempty"`
}

type UserExport struct {
	Username      string             `json:"username"`
	Email         string             `json:"email,omitempty"`
	FirstName     string             `json:"firstName,omitempty"`
	LastName      string             `json:"lastName,omitempty"`
	Enabled       *bool              `json:"enabled,omitempty"`
	EmailVerified *bool              `json:"emailVerified,omitempty"`
	Groups        []string           `json:"groups,omitempty"`
	Credentials   []CredentialExport `json:"credentials,omitempty"`
}

type CredentialExport struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

type GroupExport struct {
	Name       string              `json:"name"`
	Path       string              `json:"path,omitempty"`
	Attributes map[string][]string `json:"attributes,omitempty"`
	RealmRoles []string            `json:"realmRoles,omitempty"`
}

type ClientScopeExport struct {
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol,omitempty"`
	Description string            `json:"description,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type AuthFlowExport struct {
	Alias                    string                `json:"alias"`
	Description              string                `json:"description,omitempty"`
	ProviderID               string                `json:"providerId,omitempty"`
	TopLevel                 bool                  `json:"topLevel"`
	BuiltIn                  bool                  `json:"builtIn"`
	AuthenticationExecutions []AuthExecutionExport `json:"authenticationExecutions,omitempty"`
}

type AuthExecutionExport struct {
	Authenticator string `json:"authenticator,omitempty"`
	Requirement   string `json:"requirement,omitempty"`
	Priority      int    `json:"priority,omitempty"`
}

type IdentityProviderExport struct {
	Alias                     string            `json:"alias"`
	DisplayName               string            `json:"displayName,omitempty"`
	ProviderID                string            `json:"providerId"`
	Enabled                   bool              `json:"enabled,omitempty"`
	StoreToken                bool              `json:"storeToken,omitempty"`
	AddReadTokenRoleOnCreate  bool              `json:"addReadTokenRoleOnCreate,omitempty"`
	TrustEmail                bool              `json:"trustEmail,omitempty"`
	LinkOnly                  bool              `json:"linkOnly,omitempty"`
	FirstBrokerLoginFlowAlias string            `json:"firstBrokerLoginFlowAlias,omitempty"`
	PostBrokerLoginFlowAlias  string            `json:"postBrokerLoginFlowAlias,omitempty"`
	Config                    map[string]string `json:"config,omitempty"`
}

// BuildRealmExport generates the full JSON-compatible struct for keycloak-config-cli.
func BuildRealmExport(ctx context.Context, c client.Client, namespace string, realm v1alpha1.Realm) (*RealmExport, error) {
	realmName := realm.Spec.RealmName
	if realmName == "" {
		realmName = "master" // fallback though CRD should enforce it
	}

	export := &RealmExport{
		Realm:                realmName,
		DisplayName:          realm.Spec.DisplayName,
		Enabled:              realm.Spec.Enabled,
		RegistrationAllowed:  realm.Spec.RegistrationAllowed,
		ResetPasswordAllowed: realm.Spec.ResetPasswordAllowed,
		BruteForceProtected:  realm.Spec.BruteForceProtected,
		LoginTheme:           realm.Spec.LoginTheme,
		AccessTokenLifespan:  realm.Spec.AccessTokenLifespan,
		Clients:              []ClientExport{},
		Users:                []UserExport{},
		Groups:               []GroupExport{},
		ClientScopes:         []ClientScopeExport{},
		AuthenticationFlows:  []AuthFlowExport{},
		IdentityProviders:    []IdentityProviderExport{},
	}

	// 1. Clients
	var clientList v1alpha1.ClientList
	if err := c.List(ctx, &clientList, client.InNamespace(namespace)); err == nil {
		for _, item := range clientList.Items {
			if !item.DeletionTimestamp.IsZero() {
				continue
			}
			if getRealmRef(item.Spec.RealmRef) == realmName {
				cEx := ClientExport{
					ClientId:     item.Spec.ClientID,
					Name:         item.Spec.Name,
					Description:  item.Spec.Description,
					Enabled:      item.Spec.Enabled,
					Protocol:     item.Spec.Protocol,
					PublicClient: item.Spec.PublicClient,
					RedirectUris: item.Spec.RedirectUris,
					WebOrigins:   item.Spec.WebOrigins,
				}

				if item.Spec.PublicClient == nil || !*item.Spec.PublicClient {
					var secret corev1.Secret
					secretName := types.NamespacedName{Namespace: namespace, Name: item.Spec.ClientID + "-secret"}
					if err := c.Get(ctx, secretName, &secret); err == nil {
						if val, ok := secret.Data["CLIENT_SECRET"]; ok {
							cEx.Secret = string(val)
						}
					}
				}
				export.Clients = append(export.Clients, cEx)
			}
		}
	}

	// 2. Client Scopes
	var scopeList v1alpha1.ClientScopeList
	if err := c.List(ctx, &scopeList, client.InNamespace(namespace)); err == nil {
		for _, item := range scopeList.Items {
			if !item.DeletionTimestamp.IsZero() {
				continue
			}
			if getRealmRef(item.Spec.RealmRef) == realmName {
				export.ClientScopes = append(export.ClientScopes, ClientScopeExport{
					Name:        item.Spec.Name,
					Protocol:    item.Spec.Protocol,
					Description: item.Spec.Description,
					Attributes:  item.Spec.Attributes,
				})
			}
		}
	}

	// 3. Groups
	var groupList v1alpha1.GroupList
	if err := c.List(ctx, &groupList, client.InNamespace(namespace)); err == nil {
		for _, item := range groupList.Items {
			if !item.DeletionTimestamp.IsZero() {
				continue
			}
			if getRealmRef(item.Spec.RealmRef) == realmName {
				export.Groups = append(export.Groups, GroupExport{
					Name:       item.Spec.Name,
					Path:       item.Spec.Path,
					Attributes: item.Spec.Attributes,
					RealmRoles: item.Spec.RealmRoles,
				})
			}
		}
	}

	// 4. Users
	var userList v1alpha1.UserList
	if err := c.List(ctx, &userList, client.InNamespace(namespace)); err == nil {
		for _, item := range userList.Items {
			if !item.DeletionTimestamp.IsZero() {
				continue
			}
			if getRealmRef(item.Spec.RealmRef) == realmName {
				userEx := UserExport{
					Username:      item.Spec.Username,
					Email:         item.Spec.Email,
					FirstName:     item.Spec.FirstName,
					LastName:      item.Spec.LastName,
					Enabled:       item.Spec.Enabled,
					EmailVerified: item.Spec.EmailVerified,
					Groups:        item.Spec.Groups,
				}

				if item.Spec.InitialPassword != nil {
					var secret corev1.Secret
					secretName := types.NamespacedName{Namespace: namespace, Name: item.Spec.InitialPassword.SecretName}
					if err := c.Get(ctx, secretName, &secret); err == nil {
						key := item.Spec.InitialPassword.SecretKey
						if key == "" {
							key = "password"
						}
						if val, ok := secret.Data[key]; ok {
							userEx.Credentials = append(userEx.Credentials, CredentialExport{
								Type:      "password",
								Value:     string(val),
								Temporary: true,
							})
						}
					}
				}
				export.Users = append(export.Users, userEx)
			}
		}
	}

	// 5. Authentication Flows
	var flowList v1alpha1.AuthFlowList
	if err := c.List(ctx, &flowList, client.InNamespace(namespace)); err == nil {
		for _, item := range flowList.Items {
			if !item.DeletionTimestamp.IsZero() {
				continue
			}
			if getRealmRef(item.Spec.RealmRef) == realmName {
				flowEx := AuthFlowExport{
					Alias:       item.Spec.Alias,
					Description: item.Spec.Description,
					ProviderID:  "basic-flow",
					TopLevel:    item.Spec.TopLevel,
					BuiltIn:     false,
				}
				
				// Translate Defense Profile Toggles to executions
				flowEx.AuthenticationExecutions = append(flowEx.AuthenticationExecutions, AuthExecutionExport{
					Authenticator: "auth-username-password-form",
					Requirement:   "REQUIRED",
					Priority:      10,
				})

				if item.Spec.RequireMFA {
					flowEx.AuthenticationExecutions = append(flowEx.AuthenticationExecutions, AuthExecutionExport{
						Authenticator: "auth-otp-form",
						Requirement:   "REQUIRED",
						Priority:      20,
					})
				}

				export.AuthenticationFlows = append(export.AuthenticationFlows, flowEx)
			}
		}
	}

	// 6. Identity Providers
	var idpList v1alpha1.IdentityProviderList
	if err := c.List(ctx, &idpList, client.InNamespace(namespace)); err == nil {
		for _, item := range idpList.Items {
			if !item.DeletionTimestamp.IsZero() {
				continue
			}
			if getRealmRef(item.Spec.RealmRef) == realmName {
				idpEx := IdentityProviderExport{
					Alias:                     item.Spec.Alias,
					DisplayName:               item.Spec.DisplayName,
					ProviderID:                item.Spec.Type,
					Enabled:                   item.Spec.Enabled,
					StoreToken:                item.Spec.StoreToken,
					AddReadTokenRoleOnCreate:  item.Spec.AddReadTokenRoleOnCreate,
					TrustEmail:                item.Spec.TrustEmail,
					LinkOnly:                  item.Spec.LinkOnly,
					FirstBrokerLoginFlowAlias: item.Spec.FirstBrokerLoginFlowAlias,
					PostBrokerLoginFlowAlias:  item.Spec.PostBrokerLoginFlowAlias,
					Config:                    make(map[string]string),
				}
				
				// Copy static config
				for k, v := range item.Spec.Config {
					idpEx.Config[k] = v
				}

				// Resolve ClientSecretRef securely (OIDC)
				if item.Spec.ClientSecretRef != nil {
					var secret corev1.Secret
					secretName := types.NamespacedName{Namespace: namespace, Name: item.Spec.ClientSecretRef.Name}
					if err := c.Get(ctx, secretName, &secret); err == nil {
						if val, ok := secret.Data[item.Spec.ClientSecretRef.Key]; ok {
							idpEx.Config["clientSecret"] = string(val)
						}
					}
				}

				// Resolve SigningCertificateRef securely (SAML)
				if item.Spec.SigningCertificateRef != nil {
					var secret corev1.Secret
					secretName := types.NamespacedName{Namespace: namespace, Name: item.Spec.SigningCertificateRef.Name}
					if err := c.Get(ctx, secretName, &secret); err == nil {
						if val, ok := secret.Data[item.Spec.SigningCertificateRef.Key]; ok {
							idpEx.Config["signingCertificate"] = string(val)
						}
					}
				}

				export.IdentityProviders = append(export.IdentityProviders, idpEx)
			}
		}
	}

	return export, nil
}

func getRealmRef(ref string) string {
	if ref == "" {
		return "master"
	}
	return ref
}

func ptrToInt(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}
