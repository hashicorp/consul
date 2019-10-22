// +build !consulent

package acl

// EnterpriseRule stub
type EnterpriseRule struct{}

func (r *EnterpriseRule) Validate(string, *EnterpriseACLConfig) error {
	// nothing to validate
	return nil
}

// EnterprisePolicyRules stub
type EnterprisePolicyRules struct{}

func (r *EnterprisePolicyRules) Validate(*EnterpriseACLConfig) error {
	// nothing to validate
	return nil
}
