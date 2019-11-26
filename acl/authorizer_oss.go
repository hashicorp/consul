// +build !consulent

package acl

// EnterpriseAuthorizerContext stub
type EnterpriseAuthorizerContext struct{}

// EnterpriseAuthorizer stub interface
type EnterpriseAuthorizer interface{}

func EnforceEnterprise(_ Authorizer, _ Resource, _ string, _ string, _ *EnterpriseAuthorizerContext) (bool, EnforcementDecision, error) {
	return false, Deny, nil
}
