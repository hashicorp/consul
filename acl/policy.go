package acl

import (
	"fmt"

	"github.com/hashicorp/hcl"
)

const (
	KeyPolicyDeny      = "deny"
	KeyPolicyRead      = "read"
	KeyPolicyWrite     = "write"
	ServicePolicyDeny  = "deny"
	ServicePolicyRead  = "read"
	ServicePolicyWrite = "write"
)

// Policy is used to represent the policy specified by
// an ACL configuration.
type Policy struct {
	ID       string           `hcl:"-"`
	Keys     []*KeyPolicy     `hcl:"key,expand"`
	Services []*ServicePolicy `hcl:"service,expand"`
}

// KeyPolicy represents a policy for a key
type KeyPolicy struct {
	Prefix string `hcl:",key"`
	Policy string
}

func (k *KeyPolicy) GoString() string {
	return fmt.Sprintf("%#v", *k)
}

// ServicePolicy represents a policy for a service
type ServicePolicy struct {
	Name   string `hcl:",key"`
	Policy string
}

func (k *ServicePolicy) GoString() string {
	return fmt.Sprintf("%#v", *k)
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

	// Validate the service policy
	for _, sp := range p.Services {
		switch sp.Policy {
		case ServicePolicyDeny:
		case ServicePolicyRead:
		case ServicePolicyWrite:
		default:
			return nil, fmt.Errorf("Invalid service policy: %#v", sp)
		}
	}

	return p, nil
}
