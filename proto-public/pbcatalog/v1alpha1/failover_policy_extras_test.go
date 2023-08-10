// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
)

func TestFailoverPolicy_IsEmpty(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var fc *FailoverConfig
		require.True(t, fc.IsEmpty())
	})
	t.Run("empty", func(t *testing.T) {
		fc := &FailoverConfig{}
		require.True(t, fc.IsEmpty())
	})
	t.Run("dest", func(t *testing.T) {
		fc := &FailoverConfig{
			Destinations: []*FailoverDestination{
				newFailoverDestination("foo"),
			},
		}
		require.False(t, fc.IsEmpty())
	})
	t.Run("regions", func(t *testing.T) {
		fc := &FailoverConfig{
			Regions: []string{"us-east"},
		}
		require.False(t, fc.IsEmpty())
	})
	t.Run("regions", func(t *testing.T) {
		fc := &FailoverConfig{
			SamenessGroup: "blah",
		}
		require.False(t, fc.IsEmpty())
	})
}

func TestFailoverPolicy_GetUnderlyingDestinations_AndRefs(t *testing.T) {
	type testcase struct {
		failover    *FailoverPolicy
		expectDests []*FailoverDestination
		expectRefs  []*pbresource.Reference
	}

	run := func(t *testing.T, tc testcase) {
		assertSliceEquals(t, tc.expectDests, tc.failover.GetUnderlyingDestinations())
		assertSliceEquals(t, tc.expectRefs, tc.failover.GetUnderlyingDestinationRefs())
	}

	cases := map[string]testcase{
		"nil": {},
		"kitchen sink dests": {
			failover: &FailoverPolicy{
				Config: &FailoverConfig{
					Destinations: []*FailoverDestination{
						newFailoverDestination("foo"),
						newFailoverDestination("bar"),
					},
				},
				PortConfigs: map[string]*FailoverConfig{
					"admin": {
						Destinations: []*FailoverDestination{
							newFailoverDestination("admin"),
						},
					},
					"web": {
						Destinations: []*FailoverDestination{
							newFailoverDestination("foo"), // duplicated
							newFailoverDestination("www"),
						},
					},
				},
			},
			expectDests: []*FailoverDestination{
				newFailoverDestination("foo"),
				newFailoverDestination("bar"),
				newFailoverDestination("admin"),
				newFailoverDestination("foo"), // duplicated
				newFailoverDestination("www"),
			},
			expectRefs: []*pbresource.Reference{
				newFailoverRef("foo"),
				newFailoverRef("bar"),
				newFailoverRef("admin"),
				newFailoverRef("foo"), // duplicated
				newFailoverRef("www"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func assertSliceEquals[V proto.Message](t *testing.T, expect, got []V) {
	t.Helper()

	require.Len(t, got, len(expect))

	// O(N*M) scan
	var expectedMissing []string
	for _, expectVal := range expect {
		found := false
		for j, gotVal := range got {
			if proto.Equal(expectVal, gotVal) {
				found = true
				got = append(got[:j], got[j+1:]...) // remove found item
				break
			}
		}

		if !found {
			expectedMissing = append(expectedMissing, protoToString(t, expectVal))
		}
	}

	if len(expectedMissing) > 0 || len(got) > 0 {
		var gotMissing []string
		for _, gotVal := range got {
			gotMissing = append(gotMissing, protoToString(t, gotVal))
		}

		t.Fatalf("assertion failed: unmatched values\n\texpected: %s\n\tactual: %s",
			expectedMissing,
			gotMissing,
		)
	}
}

func protoToString[V proto.Message](t *testing.T, pb V) string {
	m := protojson.MarshalOptions{
		Indent: "  ",
	}
	gotJSON, err := m.Marshal(pb)
	require.NoError(t, err)
	return string(gotJSON)
}

func newFailoverRef(name string) *pbresource.Reference {
	return &pbresource.Reference{
		Type: &pbresource.Type{
			Group:        "fake",
			GroupVersion: "v1alpha1",
			Kind:         "fake",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		},
		Name: name,
	}
}

func newFailoverDestination(name string) *FailoverDestination {
	return &FailoverDestination{
		Ref: newFailoverRef(name),
	}
}
