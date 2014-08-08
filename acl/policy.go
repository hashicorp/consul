package acl

import (
	"fmt"
	"github.com/hashicorp/hcl"
)

// KeyPolicyType controls the various access levels for keys
type KeyPolicyType string

const (
	KeyPolicyDeny  = "deny"
	KeyPolicyRead  = "read"
	KeyPolicyWrite = "write"
)

// Policy is used to represent the policy specified by
// an ACL configuration.
type Policy struct {
	Keys []*KeyPolicy `hcl:"key"`
}

// KeyPolicy represents a policy for a key
type KeyPolicy struct {
	Prefix string `hcl:",key"`
	Policy KeyPolicyType
}

// Parse is used to parse the specified ACL rules into an
// intermediary set of policies, before being compiled into
// the ACL
func Parse(rules string) (*Policy, error) {
	// Decode the rules
	p := &Policy{}
	if rules == "" {
		// Hot path for empty rules
		return p, nil
	}

	if err := hcl.Decode(p, rules); err != nil {
		return nil, fmt.Errorf("Failed to parse ACL rules: %v", err)
	}

	// Validate the key policy
	for _, kp := range p.Keys {
		switch kp.Policy {
		case KeyPolicyDeny:
		case KeyPolicyRead:
		case KeyPolicyWrite:
		default:
			return nil, fmt.Errorf("Invalid key policy: %#v", kp)
		}
	}
	return p, nil
}
