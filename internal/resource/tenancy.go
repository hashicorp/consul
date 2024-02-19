// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TenancyBridge interface {
	PartitionExists(partition string) (bool, error)
	IsPartitionMarkedForDeletion(partition string) (bool, error)
	NamespaceExists(partition, namespace string) (bool, error)
	IsNamespaceMarkedForDeletion(partition, namespace string) (bool, error)
}

const (
	DefaultPartitionName = "default"
	DefaultNamespaceName = "default"
	DefaultPeerName      = "local"
)

// Scope describes the tenancy scope of a resource.
type Scope int

const (
	// There is no default scope, it must be set explicitly.
	ScopeUndefined Scope = iota
	// ScopeCluster describes a resource that is scoped to a cluster.
	ScopeCluster
	// ScopePartition describes a resource that is scoped to a partition.
	ScopePartition
	// ScopeNamespace applies to a resource that is scoped to a partition and namespace.
	ScopeNamespace
)

func (s Scope) String() string {
	switch s {
	case ScopeUndefined:
		return "undefined"
	case ScopeCluster:
		return "cluster"
	case ScopePartition:
		return "partition"
	case ScopeNamespace:
		return "namespace"
	}
	panic(fmt.Sprintf("string mapping missing for scope %v", int(s)))
}

// DefaultClusteredTenancy returns the default tenancy for a cluster scoped resource.
func DefaultClusteredTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{}
}

// DefaultPartitionedTenancy returns the default tenancy for a partition scoped resource.
func DefaultPartitionedTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: DefaultPartitionName,
	}
}

// DefaultNamespedTenancy returns the default tenancy for a namespace scoped resource.
func DefaultNamespacedTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: DefaultPartitionName,
		Namespace: DefaultNamespaceName,
	}
}

// DefaultIDTenancy will default/normalize the Tenancy of the provided
// ID in the context of some parent resource containing that ID.
// The default tenancy for the ID's type is also provided in cases where
// "default" is needed selectively or the parent is more precise than the
// child.
func DefaultIDTenancy(id *pbresource.ID, parentTenancy, scopeTenancy *pbresource.Tenancy) {
	if id == nil {
		return
	}
	if id.Tenancy == nil {
		id.Tenancy = &pbresource.Tenancy{}
	}

	if parentTenancy != nil {
		dup := proto.Clone(parentTenancy).(*pbresource.Tenancy)
		parentTenancy = dup
	}

	defaultTenancy(id.Tenancy, parentTenancy, scopeTenancy)
}

// DefaultReferenceTenancy will default/normalize the Tenancy of the provided
// Reference in the context of some parent resource containing that Reference.
// The default tenancy for the Reference's type is also provided in cases where
// "default" is needed selectively or the parent is more precise than the
// child.
func DefaultReferenceTenancy(ref *pbresource.Reference, parentTenancy, scopeTenancy *pbresource.Tenancy) {
	if ref == nil {
		return
	}
	if ref.Tenancy == nil {
		ref.Tenancy = &pbresource.Tenancy{}
	}

	if parentTenancy != nil {
		dup := proto.Clone(parentTenancy).(*pbresource.Tenancy)
		parentTenancy = dup
	}

	defaultTenancy(ref.Tenancy, parentTenancy, scopeTenancy)
}

func defaultTenancy(itemTenancy, parentTenancy, scopeTenancy *pbresource.Tenancy) {
	if itemTenancy == nil {
		panic("item tenancy is required")
	}
	if scopeTenancy == nil {
		panic("scope tenancy is required")
	}

	if parentTenancy != nil {
		// Recursively normalize this tenancy as well.
		defaultTenancy(parentTenancy, nil, scopeTenancy)
	}

	// use scope defaults for parent
	if parentTenancy == nil {
		parentTenancy = scopeTenancy
	}

	// Only retain the parts of the parent that apply to this resource.
	if scopeTenancy.Partition == "" {
		parentTenancy.Partition = ""
		itemTenancy.Partition = ""
	}
	if scopeTenancy.Namespace == "" {
		parentTenancy.Namespace = ""
		itemTenancy.Namespace = ""
	}

	if parentTenancy.Partition == "" {
		// (cluster scoped)
	} else {
		if itemTenancy.Partition == "" {
			itemTenancy.Partition = parentTenancy.Partition
		}
		if parentTenancy.Namespace == "" {
			// (partition scoped)
		} else {
			// (namespace scoped)

			if itemTenancy.Namespace == "" {
				if itemTenancy.Partition == parentTenancy.Partition {
					// safe to copy the namespace
					itemTenancy.Namespace = parentTenancy.Namespace
				} else {
					// cross-peer, the namespace must come from the scope default
					itemTenancy.Namespace = scopeTenancy.Namespace
				}
			}
		}
	}
}

func equalOrEmpty(a, b string) bool {
	return (a == b) || (a == "") || (b == "")
}
