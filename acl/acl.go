package acl

import (
	"fmt"

	"github.com/armon/go-radix"
)

var (
	// allowAll is a singleton policy which allows all actions
	allowAll ACL

	// denyAll is a singleton policy which denies all actions
	denyAll ACL
)

func init() {
	// Setup the singletons
	allowAll = &StaticACL{defaultAllow: true}
	denyAll = &StaticACL{defaultAllow: false}
}

// ACL is the interface for policy enforcement.
type ACL interface {
	KeyRead(string) bool
	KeyWrite(string) bool
}

// StaticACL is used to implement a base ACL policy. It either
// allows or denies all requests. This can be used as a parent
// ACL to act in a blacklist or whitelist mode.
type StaticACL struct {
	defaultAllow bool
}

func (s *StaticACL) KeyRead(string) bool {
	return s.defaultAllow
}

func (s *StaticACL) KeyWrite(string) bool {
	return s.defaultAllow
}

// AllowAll returns an ACL rule that allows all operations
func AllowAll() ACL {
	return allowAll
}

// DenyAll returns an ACL rule that denies all operations
func DenyAll() ACL {
	return denyAll
}

// PolicyACL is used to wrap a set of ACL policies to provide
// the ACL interface.
type PolicyACL struct {
	// parent is used to resolve policy if we have
	// no matching rule.
	parent ACL

	// keyRead contains the read policies
	keyRead *radix.Tree

	// keyWrite contains the write policies
	keyWrite *radix.Tree
}

// New is used to construct a policy based ACL from a set of policies
// and a parent policy to resolve missing cases.
func New(parent ACL, policy *Policy) (*PolicyACL, error) {
	p := &PolicyACL{
		parent:   parent,
		keyRead:  radix.New(),
		keyWrite: radix.New(),
	}

	// Load the key policy
	for _, kp := range policy.Keys {
		switch kp.Policy {
		case KeyPolicyDeny:
			p.keyRead.Insert(kp.Prefix, false)
			p.keyWrite.Insert(kp.Prefix, false)
		case KeyPolicyRead:
			p.keyRead.Insert(kp.Prefix, true)
			p.keyWrite.Insert(kp.Prefix, false)
		case KeyPolicyWrite:
			p.keyRead.Insert(kp.Prefix, true)
			p.keyWrite.Insert(kp.Prefix, true)
		default:
			return nil, fmt.Errorf("Invalid key policy: %#v", kp)
		}
	}
	return p, nil
}

// KeyRead returns if a key is allowed to be read
func (p *PolicyACL) KeyRead(key string) bool {
	// Look for a matching rule
	_, rule, ok := p.keyRead.LongestPrefix(key)
	if ok {
		return rule.(bool)
	}

	// No matching rule, use the parent.
	return p.parent.KeyRead(key)
}

// KeyWrite returns if a key is allowed to be written
func (p *PolicyACL) KeyWrite(key string) bool {
	// Look for a matching rule
	_, rule, ok := p.keyWrite.LongestPrefix(key)
	if ok {
		return rule.(bool)
	}

	// No matching rule, use the parent.
	return p.parent.KeyWrite(key)
}
