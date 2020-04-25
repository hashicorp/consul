// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

// FillAuthzContext can fill in an acl.AuthorizerContext object to setup
// extra parameters for ACL enforcement. In OSS there is currently nothing
// extra to be done.
func (_ *Intention) FillAuthzContext(_ *acl.AuthorizerContext, _ bool) {
	// do nothing
}

// FillAuthzContext can fill in an acl.AuthorizerContext object to setup
// extra parameters for ACL enforcement. In OSS there is currently nothing
// extra to be done.
func (_ *IntentionMatchEntry) FillAuthzContext(_ *acl.AuthorizerContext) {
	// do nothing
}

// FillAuthzContext can fill in an acl.AuthorizerContext object to setup
// extra parameters for ACL enforcement. In OSS there is currently nothing
// extra to be done.
func (_ *IntentionQueryCheck) FillAuthzContext(_ *acl.AuthorizerContext) {
	// do nothing
}

// DefaultNamespaces will populate both the SourceNS and DestinationNS fields
// if they are empty with the proper defaults.
func (ixn *Intention) DefaultNamespaces(_ *EnterpriseMeta) {
	// Until we support namespaces, we force all namespaces to be default
	if ixn.SourceNS == "" {
		ixn.SourceNS = IntentionDefaultNamespace
	}
	if ixn.DestinationNS == "" {
		ixn.DestinationNS = IntentionDefaultNamespace
	}
}
