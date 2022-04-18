//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

func (ixn *Intention) SourceEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (ixn *Intention) DestinationEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (e *IntentionMatchEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (e *IntentionQueryExact) SourceEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (e *IntentionQueryExact) DestinationEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

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

// FillPartitionAndNamespace will fill in empty source and destination partition/namespaces.
// If fillDefault is true, all fields are defaulted when the given enterprise meta does not
// specify them.
//
// fillDefault MUST be true on servers to ensure that all fields are populated on writes.
// fillDefault MUST be false on clients so that servers can correctly fill in the
// namespace/partition of the ACL token.
func (ixn *Intention) FillPartitionAndNamespace(entMeta *acl.EnterpriseMeta, fillDefault bool) {
	if ixn == nil {
		return
	}
	var ns = entMeta.NamespaceOrEmpty()
	if fillDefault {
		if ns == "" {
			ns = IntentionDefaultNamespace
		}
	}
	if ixn.SourceNS == "" {
		ixn.SourceNS = ns
	}
	if ixn.DestinationNS == "" {
		ixn.DestinationNS = ns
	}

	ixn.SourcePartition = ""
	ixn.DestinationPartition = ""
}
