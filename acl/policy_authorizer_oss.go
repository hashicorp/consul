// +build !consulent

package acl

// enterprisePolicyAuthorizer stub
type enterprisePolicyAuthorizer struct{}

func (authz *enterprisePolicyAuthorizer) init(*EnterpriseACLConfig) {
	// nothing to do
}

func (authz *enterprisePolicyAuthorizer) enforce(_ *EnterpriseRule, _ *EnterpriseAuthorizerContext) EnforcementDecision {
	return Default
}

// NewPolicyAuthorizer merges the policies and returns an Authorizer that will enforce them
func NewPolicyAuthorizer(policies []*Policy, entConfig *EnterpriseACLConfig) (Authorizer, error) {
	return newPolicyAuthorizer(policies, entConfig)
}

// NewPolicyAuthorizerWithDefaults will actually created a ChainedAuthorizer with
// the policies compiled into one Authorizer and the backup policy of the defaultAuthz
func NewPolicyAuthorizerWithDefaults(defaultAuthz Authorizer, policies []*Policy, entConfig *EnterpriseACLConfig) (Authorizer, error) {
	authz, err := newPolicyAuthorizer(policies, entConfig)
	if err != nil {
		return nil, err
	}

	return NewChainedAuthorizer([]Authorizer{authz, defaultAuthz}), nil
}
