// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"github.com/hashicorp/consul/agent/structs"
	"strings"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TestTenancies returns a list of tenancies which represent
// the namespace and partition combinations that can be used in unit tests
func TestTenancies() []*pbresource.Tenancy {
	isEnterprise := structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty() == "default"

	tenancies := []*pbresource.Tenancy{Tenancy("default.default")}
	if isEnterprise {
		tenancies = append(tenancies, Tenancy("default.bar"), Tenancy("foo.default"), Tenancy("foo.bar"))
	}

	return tenancies
}

// Tenancy constructs a pbresource.Tenancy from a concise string representation
// suitable for use in unit tests.
//
// - ""        : partition=""    namespace=""    peerName="local"
// - "foo"     : partition="foo" namespace=""    peerName="local"
// - "foo.bar" : partition="foo" namespace="bar" peerName="local"
// - <others>  : partition="BAD" namespace="BAD" peerName="BAD"
func Tenancy(s string) *pbresource.Tenancy {
	parts := strings.Split(s, ".")
	switch len(parts) {
	case 0:
		return resource.DefaultClusteredTenancy()
	case 1:
		v := resource.DefaultPartitionedTenancy()
		v.Partition = parts[0]
		return v
	case 2:
		v := resource.DefaultNamespacedTenancy()
		v.Partition = parts[0]
		v.Namespace = parts[1]
		return v
	default:
		return &pbresource.Tenancy{Partition: "BAD", Namespace: "BAD", PeerName: "BAD"}
	}
}

func DefaultTenancyForType(t *testing.T, reg resource.Registration) *pbresource.Tenancy {
	switch reg.Scope {
	case resource.ScopeNamespace:
		return resource.DefaultNamespacedTenancy()
	case resource.ScopePartition:
		return resource.DefaultPartitionedTenancy()
	case resource.ScopeCluster:
		return resource.DefaultClusteredTenancy()
	default:
		t.Fatalf("unsupported resource scope: %v", reg.Scope)
		return nil
	}
}

// TestTenancies returns a list of tenancies which represent
// the namespace and partition combinations that can be used in unit tests
func TestTenancies() []*pbresource.Tenancy {
	isEnterprise := structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty() == "default"

	tenancies := []*pbresource.Tenancy{Tenancy("default.default")}
	if isEnterprise {
		tenancies = append(tenancies, Tenancy("default.bar"), Tenancy("foo.default"), Tenancy("foo.bar"))
	}

	return tenancies
}
