// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/version/versiontest"
)

// TestTenancies returns a list of tenancies which represent
// the namespace and partition combinations that can be used in unit tests
func TestTenancies() []*pbresource.Tenancy {
	isEnterprise := versiontest.IsEnterprise()

	tenancies := []*pbresource.Tenancy{Tenancy("default.default")}
	if isEnterprise {
		// TODO(namespaces/v2) move the default partition + non-default namespace test to run even for CE.
		tenancies = append(tenancies, Tenancy("default.bar"), Tenancy("foo.default"), Tenancy("foo.bar"))
	}

	return tenancies
}

// Tenancy constructs a pbresource.Tenancy from a concise string representation
// suitable for use in unit tests.
//
// - ""        : partition=""    namespace=""
// - "foo"     : partition="foo" namespace=""
// - "foo.bar" : partition="foo" namespace="bar"
// - <others>  : partition="BAD" namespace="BAD"
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
		return &pbresource.Tenancy{Partition: "BAD", Namespace: "BAD"}
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

func AppendTenancyInfoSubtest(name string, subtestName string, tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_%s_Namespace_%s_Partition/%s", name, tenancy.Namespace, tenancy.Partition, subtestName)
}

func AppendTenancyInfo(name string, tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_%s_Namespace_%s_Partition", name, tenancy.Namespace, tenancy.Partition)
}

func RunWithTenancies(testFunc func(tenancy *pbresource.Tenancy), t *testing.T) {
	for _, tenancy := range TestTenancies() {
		t.Run(AppendTenancyInfo(t.Name(), tenancy), func(t *testing.T) {
			testFunc(tenancy)
		})
	}
}
