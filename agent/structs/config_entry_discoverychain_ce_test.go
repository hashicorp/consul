// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceResolverConfigEntry_OSS(t *testing.T) {
	type testcase struct {
		name         string
		entry        *ServiceResolverConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceResolverConfigEntry)
	}

	cases := []testcase{
		{
			name: "failover with a sameness group on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						SamenessGroup: "ns1",
					},
				},
			},
			validateErr: `Bad Failover["*"]: Setting SamenessGroup requires Consul Enterprise`,
		},
		{
			name: "failover with a namespace on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Service:   "backup",
						Namespace: "ns1",
					},
				},
			},
			validateErr: `Bad Failover["*"]: Setting Namespace requires Consul Enterprise`,
		},
		{
			name: "failover Targets cannot set Namespace on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Targets: []ServiceResolverFailoverTarget{{Namespace: "ns1"}},
					},
				},
			},
			validateErr: `Bad Failover["*"].Targets[0]: Setting Namespace requires Consul Enterprise`,
		},
		{
			name: "failover Targets cannot set Partition on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Targets: []ServiceResolverFailoverTarget{{Partition: "ap1"}},
					},
				},
			},
			validateErr: `Bad Failover["*"].Targets[0]: Setting Partition requires Consul Enterprise`,
		},
		{
			name: "setting failover Namespace on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {Namespace: "ns1"},
				},
			},
			validateErr: `Bad Failover["*"]: Setting Namespace requires Consul Enterprise`,
		},
		{
			name: "setting failover Namespace on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {Service: "s1", Policy: &ServiceResolverFailoverPolicy{Mode: "something"}},
				},
			},
			validateErr: `Bad Failover["*"]: Setting failover policies requires Consul Enterprise`,
		},
		{
			name: "setting redirect SamenessGroup on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					SamenessGroup: "group",
				},
			},
			validateErr: `Redirect: Setting SamenessGroup requires Consul Enterprise`,
		},
		{
			name: "setting redirect Namespace on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Namespace: "ns1",
				},
			},
			validateErr: `Redirect: Setting Namespace requires Consul Enterprise`,
		},
		{
			name: "setting redirect Partition on OSS",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Partition: "ap1",
				},
			},
			validateErr: `Redirect: Setting Partition requires Consul Enterprise`,
		},
	}

	// Bulk add a bunch of similar validation cases.
	for _, invalidSubset := range invalidSubsetNames {
		tc := testcase{
			name: "invalid subset name: " + invalidSubset,
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					invalidSubset: {OnlyPassing: true},
				},
			},
			validateErr: fmt.Sprintf("Subset %q is invalid", invalidSubset),
		}
		cases = append(cases, tc)
	}

	for _, goodSubset := range validSubsetNames {
		tc := testcase{
			name: "valid subset name: " + goodSubset,
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					goodSubset: {OnlyPassing: true},
				},
			},
		}
		cases = append(cases, tc)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
