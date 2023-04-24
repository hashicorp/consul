//go:build !consulent
// +build !consulent

package acl

// AuthorizerContext stub
type AuthorizerContext struct{}

// enterpriseAuthorizer stub interface
type enterpriseAuthorizer interface{}

func enforceEnterprise(_ Authorizer, _ Resource, _ string, _ string, _ *AuthorizerContext) (bool, EnforcementDecision, error) {
	return false, Deny, nil
}
