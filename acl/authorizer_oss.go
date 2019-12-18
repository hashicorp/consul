// +build !consulent

package acl

// AuthorizerContext stub
type AuthorizerContext struct{}

// EnterpriseAuthorizer stub interface
type EnterpriseAuthorizer interface{}

func EnforceEnterprise(_ Authorizer, _ Resource, _ string, _ string, _ *AuthorizerContext) (bool, EnforcementDecision, error) {
	return false, Deny, nil
}
