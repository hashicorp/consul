// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	DefaultPartitionName = "default"
	DefaultNamespaceName = "default"
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

// Normalize lowercases the partition and namespace.
func Normalize(tenancy *pbresource.Tenancy) {
	if tenancy == nil {
		return
	}
	tenancy.Partition = strings.ToLower(tenancy.Partition)
	tenancy.Namespace = strings.ToLower(tenancy.Namespace)
}

// DefaultClusteredTenancy returns the default tenancy for a cluster scoped resource.
func DefaultClusteredTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		// TODO(spatel): Remove as part of "peer is not part of tenancy" ADR
		PeerName: "local",
	}
}

// DefaultPartitionedTenancy returns the default tenancy for a partition scoped resource.
func DefaultPartitionedTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: DefaultPartitionName,
		// TODO(spatel): Remove as part of "peer is not part of tenancy" ADR
		PeerName: "local",
	}
}

// DefaultNamespedTenancy returns the default tenancy for a namespace scoped resource.
func DefaultNamespacedTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: DefaultPartitionName,
		Namespace: DefaultNamespaceName,
		// TODO(spatel): Remove as part of "peer is not part of tenancy" ADR
		PeerName: "local",
	}
}

// WildCardTenancyFor returns a valid tenancy with tenancy units set to wildcard for
// the passed in scope.
func WildCardTenacyFor(scope Scope) *pbresource.Tenancy {
	switch scope {
	case ScopePartition:
		return &pbresource.Tenancy{
			Partition: storage.Wildcard,
			// TODO(spatel): Remove PeerName once peer tenancy introduced
			PeerName: storage.Wildcard,
		}
	case ScopeNamespace:
		return &pbresource.Tenancy{
			Partition: storage.Wildcard,
			Namespace: storage.Wildcard,
			// TODO(spatel): Remove PeerName once peer tenancy introduced
			PeerName: storage.Wildcard,
		}
	default:
		// Resouces like tombstones which have undefined scope should return
		// same wildcard tenancy as namespaced resources for now.
		return &pbresource.Tenancy{
			Partition: storage.Wildcard,
			Namespace: storage.Wildcard,
			// TODO(spatel): Remove PeerName once peer tenancy introduced
			PeerName: storage.Wildcard,
		}
	}
}

// NamespaceToPartitionTenancy creates a partition scoped tenancy from an existing
// namespace scoped tenancy.
func NamespaceToPartitionTenancy(nt *pbresource.Tenancy) *pbresource.Tenancy {
	pt := DefaultPartitionedTenancy()
	pt.Partition = nt.Partition
	return pt
}

// DefaultTenancyFor returns the default tenancy for the passed in scope.
func DefaultTenancyFor(scope Scope) *pbresource.Tenancy {
	switch scope {
	case ScopePartition:
		return DefaultPartitionedTenancy()
	case ScopeNamespace:
		return DefaultNamespacedTenancy()
	default:
		panic(fmt.Sprintf("unsupported scope: %v", scope))
	}
}
