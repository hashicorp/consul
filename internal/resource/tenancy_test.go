// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestDefaultReferenceTenancy(t *testing.T) {
	// Just do a few small tests here and let the more complicated cases be covered by
	// TestDefaultTenancy below.

	t.Run("partition inference", func(t *testing.T) {
		ref := &pbresource.Reference{
			Type: &pbresource.Type{
				Group:        "fake",
				GroupVersion: "v1fake",
				Kind:         "artificial",
			},
			Name: "blah",
			Tenancy: &pbresource.Tenancy{
				Namespace: "zim",
			},
		}

		expect := &pbresource.Reference{
			Type: &pbresource.Type{
				Group:        "fake",
				GroupVersion: "v1fake",
				Kind:         "artificial",
			},
			Name:    "blah",
			Tenancy: newTestTenancy("gir.zim"),
		}

		parent := newTestTenancy("gir.gaz")

		DefaultReferenceTenancy(ref, parent, DefaultNamespacedTenancy())
		prototest.AssertDeepEqual(t, expect, ref)
	})

	t.Run("full default", func(t *testing.T) {
		ref := &pbresource.Reference{
			Type: &pbresource.Type{
				Group:        "fake",
				GroupVersion: "v1fake",
				Kind:         "artificial",
			},
			Name: "blah",
		}

		expect := &pbresource.Reference{
			Type: &pbresource.Type{
				Group:        "fake",
				GroupVersion: "v1fake",
				Kind:         "artificial",
			},
			Name:    "blah",
			Tenancy: newTestTenancy("gir.gaz"),
		}

		parent := newTestTenancy("gir.gaz")

		DefaultReferenceTenancy(ref, parent, DefaultNamespacedTenancy())
		prototest.AssertDeepEqual(t, expect, ref)
	})
}

func TestDefaultTenancy(t *testing.T) {
	type testcase struct {
		ref    *pbresource.Tenancy
		parent *pbresource.Tenancy
		scope  *pbresource.Tenancy
		expect *pbresource.Tenancy
	}

	run := func(t *testing.T, tc testcase) {
		got := proto.Clone(tc.ref).(*pbresource.Tenancy)

		defaultTenancy(got, tc.parent, tc.scope)
		prototest.AssertDeepEqual(t, tc.expect, got)
	}

	cases := map[string]testcase{
		// Completely empty values get backfilled from the scope.
		"clustered/empty/no-parent": {
			ref:    newTestTenancy(""),
			parent: nil,
			scope:  DefaultClusteredTenancy(),
			expect: DefaultClusteredTenancy(),
		},
		"partitioned/empty/no-parent": {
			ref:    newTestTenancy(""),
			parent: nil,
			scope:  DefaultPartitionedTenancy(),
			expect: DefaultPartitionedTenancy(),
		},
		"namespaced/empty/no-parent": {
			ref:    newTestTenancy(""),
			parent: nil,
			scope:  DefaultNamespacedTenancy(),
			expect: DefaultNamespacedTenancy(),
		},
		// Completely provided values are limited by the scope.
		"clustered/full/no-parent": {
			ref:    newTestTenancy("foo.bar"),
			parent: nil,
			scope:  DefaultClusteredTenancy(),
			expect: DefaultClusteredTenancy(),
		},
		"partitioned/full/no-parent": {
			ref:    newTestTenancy("foo.bar"),
			parent: nil,
			scope:  DefaultPartitionedTenancy(),
			expect: newTestTenancy("foo"),
		},
		"namespaced/full/no-parent": {
			ref:    newTestTenancy("foo.bar"),
			parent: nil,
			scope:  DefaultNamespacedTenancy(),
			expect: newTestTenancy("foo.bar"),
		},
		// Completely provided parent values are limited by the scope before
		// being blindly used for to fill in for the empty provided value.
		"clustered/empty/full-parent": {
			ref:    newTestTenancy(""),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultClusteredTenancy(),
			expect: DefaultClusteredTenancy(),
		},
		"partitioned/empty/full-parent": {
			ref:    newTestTenancy(""),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultPartitionedTenancy(),
			expect: newTestTenancy("foo"),
		},
		"namespaced/empty/full-parent": {
			ref:    newTestTenancy(""),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultNamespacedTenancy(),
			expect: newTestTenancy("foo.bar"),
		},
		// (1) Partially filled values are only partially populated by parents.
		"clustered/part-only/full-parent": {
			ref:    newTestTenancy("zim"),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultClusteredTenancy(),
			expect: DefaultClusteredTenancy(),
		},
		"partitioned/part-only/full-parent": {
			ref:    newTestTenancy("zim"),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultPartitionedTenancy(),
			expect: newTestTenancy("zim"),
		},
		"namespaced/part-only/full-parent": {
			ref:    newTestTenancy("zim"),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultNamespacedTenancy(),
			// partitions don't match so the namespace comes from the scope
			expect: newTestTenancy("zim.default"),
		},
		// (2) Partially filled values are only partially populated by parents.
		"clustered/ns-only/full-parent": {
			// Leading dot implies no partition
			ref:    newTestTenancy(".gir"),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultClusteredTenancy(),
			expect: DefaultClusteredTenancy(),
		},
		"partitioned/ns-only/full-parent": {
			// Leading dot implies no partition
			ref:    newTestTenancy(".gir"),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultPartitionedTenancy(),
			expect: newTestTenancy("foo"),
		},
		"namespaced/ns-only/full-parent": {
			// Leading dot implies no partition
			ref:    newTestTenancy(".gir"),
			parent: newTestTenancy("foo.bar"),
			scope:  DefaultNamespacedTenancy(),
			expect: newTestTenancy("foo.gir"),
		},
		// Fully specified ignores parent.
		"clustered/full/full-parent": {
			ref:    newTestTenancy("foo.bar"),
			parent: newTestTenancy("zim.gir"),
			scope:  DefaultClusteredTenancy(),
			expect: DefaultClusteredTenancy(),
		},
		"partitioned/full/full-parent": {
			ref:    newTestTenancy("foo.bar"),
			parent: newTestTenancy("zim.gir"),
			scope:  DefaultPartitionedTenancy(),
			expect: newTestTenancy("foo"),
		},
		"namespaced/full/full-parent": {
			ref:    newTestTenancy("foo.bar"),
			parent: newTestTenancy("zim.gir"),
			scope:  DefaultNamespacedTenancy(),
			expect: newTestTenancy("foo.bar"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func newTestTenancy(s string) *pbresource.Tenancy {
	parts := strings.Split(s, ".")
	switch len(parts) {
	case 0:
		return DefaultClusteredTenancy()
	case 1:
		v := DefaultPartitionedTenancy()
		v.Partition = parts[0]
		return v
	case 2:
		v := DefaultNamespacedTenancy()
		v.Partition = parts[0]
		v.Namespace = parts[1]
		return v
	default:
		return &pbresource.Tenancy{Partition: "BAD", Namespace: "BAD"}
	}
}
