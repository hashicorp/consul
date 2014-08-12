package acl

import (
	"github.com/armon/go-radix"
)

var (
	// allowAll is a singleton policy which allows all
	// non-management actions
	allowAll ACL

	// denyAll is a singleton policy which denies all actions
	denyAll ACL

	// manageAll is a singleton policy which allows all
	// actions, including management
	manageAll ACL
)

func init() {
	// Setup the singletons
	allowAll = &StaticACL{
		allowManage:  false,
		defaultAllow: true,
	}
	denyAll = &StaticACL{
		allowManage:  false,
		defaultAllow: false,
	}
	manageAll = &StaticACL{
		allowManage:  true,
		defaultAllow: true,
	}
}

// ACL is the interface for policy enforcement.
type ACL interface {
	KeyRead(string) bool
	KeyWrite(string) bool
	ACLList() bool
	ACLModify() bool
}

// StaticACL is used to implement a base ACL policy. It either
// allows or denies all requests. This can be used as a parent
// ACL to act in a blacklist or whitelist mode.
type StaticACL struct {
	allowManage  bool
	defaultAllow bool
}

func (s *StaticACL) KeyRead(string) bool {
	return s.defaultAllow
}

func (s *StaticACL) KeyWrite(string) bool {
	return s.defaultAllow
}

func (s *StaticACL) ACLList() bool {
	return s.allowManage
}

func (s *StaticACL) ACLModify() bool {
	return s.allowManage
}

// AllowAll returns an ACL rule that allows all operations
func AllowAll() ACL {
	return allowAll
}

// DenyAll returns an ACL rule that denies all operations
func DenyAll() ACL {
	return denyAll
}

// ManageAll returns an ACL rule that can manage all resources
func ManageAll() ACL {
	return manageAll
}

// RootACL returns a possible ACL if the ID matches a root policy
func RootACL(id string) ACL {
	switch id {
	case "allow":
		return allowAll
	case "deny":
		return denyAll
	case "manage":
		return manageAll
	default:
		return nil
	}
}

// PolicyACL is used to wrap a set of ACL policies to provide
// the ACL interface.
type PolicyACL struct {
	// parent is used to resolve policy if we have
	// no matching rule.
	parent ACL

	// keyRules contains the key policies
	keyRules *radix.Tree
}

// New is used to construct a policy based ACL from a set of policies
// and a parent policy to resolve missing cases.
func New(parent ACL, policy *Policy) (*PolicyACL, error) {
	p := &PolicyACL{
		parent:   parent,
		keyRules: radix.New(),
	}

	// Load the key policy
	for _, kp := range policy.Keys {
		p.keyRules.Insert(kp.Prefix, kp.Policy)
	}
	return p, nil
}

// KeyRead returns if a key is allowed to be read
func (p *PolicyACL) KeyRead(key string) bool {
	// Look for a matching rule
	_, rule, ok := p.keyRules.LongestPrefix(key)
	if ok {
		switch rule.(string) {
		case KeyPolicyRead:
			return true
		case KeyPolicyWrite:
			return true
		default:
			return false
		}
	}

	// No matching rule, use the parent.
	return p.parent.KeyRead(key)
}

// KeyWrite returns if a key is allowed to be written
func (p *PolicyACL) KeyWrite(key string) bool {
	// Look for a matching rule
	_, rule, ok := p.keyRules.LongestPrefix(key)
	if ok {
		switch rule.(string) {
		case KeyPolicyWrite:
			return true
		default:
			return false
		}
	}

	// No matching rule, use the parent.
	return p.parent.KeyWrite(key)
}

// ACLList checks if listing of ACLs is allowed
func (p *PolicyACL) ACLList() bool {
	return p.ACLList()
}

// ACLModify checks if modification of ACLs is allowed
func (p *PolicyACL) ACLModify() bool {
	return p.ACLModify()
}
