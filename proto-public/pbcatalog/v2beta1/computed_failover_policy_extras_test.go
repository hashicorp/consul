// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

import (
	"testing"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestComputedFailoverPolicy_GetUnderlyingDestinations_AndRefs(t *testing.T) {
	type testcase struct {
		failover    *ComputedFailoverPolicy
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
			failover: &ComputedFailoverPolicy{
				PortConfigs: map[string]*FailoverConfig{
					"http": {
						Destinations: []*FailoverDestination{
							newFailoverDestination("foo"),
							newFailoverDestination("bar"),
						},
					},
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
