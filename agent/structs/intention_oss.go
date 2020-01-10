// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

func (_ *Intention) FillAuthzContext(_ *acl.AuthorizerContext, _ bool) {
	// do nothing
}

func (_ *IntentionMatchEntry) FillAuthzContext(_ *acl.AuthorizerContext) {
	// do nothing
}

func (_ *IntentionQueryCheck) FillAuthzContext(_ *acl.AuthorizerContext) {
	// do nothing
}

func (ixn *Intention) DefaultNamespaces(_ *EnterpriseMeta) {
	// Until we support namespaces, we force all namespaces to be default
	if ixn.SourceNS == "" {
		ixn.SourceNS = IntentionDefaultNamespace
	}
	if ixn.DestinationNS == "" {
		ixn.DestinationNS = IntentionDefaultNamespace
	}
}
