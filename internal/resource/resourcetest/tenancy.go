// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

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
