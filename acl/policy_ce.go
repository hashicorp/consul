//go:build !consulent
// +build !consulent

package acl

import (
	"fmt"

	"github.com/hashicorp/hcl"
)

// EnterprisePolicyMeta stub
type EnterprisePolicyMeta struct{}

// EnterpriseRule stub
type EnterpriseRule struct{}

func (r *EnterpriseRule) Validate(string, *Config) error {
	// nothing to validate
	return nil
}

// EnterprisePolicyRules stub
type EnterprisePolicyRules struct{}

func (r *EnterprisePolicyRules) Validate(*Config) error {
	// nothing to validate
	return nil
}

func decodeRules(rules string, _ *Config, _ *EnterprisePolicyMeta) (*Policy, error) {
	p := &Policy{}

	if err := hcl.Decode(p, rules); err != nil {
		return nil, fmt.Errorf("Failed to parse ACL rules: %v", err)
	}

	return p, nil
}
