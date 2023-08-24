// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package instances

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestUsageInstances_formatServiceCounts(t *testing.T) {
	usageBasic := map[string]api.ServiceUsage{
		"dc1": {
			Services:         10,
			ServiceInstances: 35,
			ConnectServiceInstances: map[string]int{
				"connect-native":      1,
				"connect-proxy":       3,
				"ingress-gateway":     4,
				"mesh-gateway":        2,
				"terminating-gateway": 5,
			},
			BillableServiceInstances: 20,
		},
	}

	usageMultiDC := map[string]api.ServiceUsage{
		"dc1": {
			Services:         10,
			ServiceInstances: 35,
			ConnectServiceInstances: map[string]int{
				"connect-native":      1,
				"connect-proxy":       3,
				"ingress-gateway":     4,
				"mesh-gateway":        2,
				"terminating-gateway": 5,
			},
			BillableServiceInstances: 20,
		},
		"dc2": {
			Services:         23,
			ServiceInstances: 11,
			ConnectServiceInstances: map[string]int{
				"connect-native":      9,
				"connect-proxy":       8,
				"ingress-gateway":     7,
				"mesh-gateway":        6,
				"terminating-gateway": 0,
			},
			BillableServiceInstances: 33,
		},
	}

	cases := []struct {
		name             string
		usageStats       map[string]api.ServiceUsage
		showDatacenter   bool
		expectedBillable string
		expectedConnect  string
	}{
		{
			name:       "basic",
			usageStats: usageBasic,
			expectedBillable: `
Services      Service instances
10            20`,
			expectedConnect: `
Type                     Service instances
connect-native           1
connect-proxy            3
ingress-gateway          4
mesh-gateway             2
terminating-gateway      5`,
		},
		{
			name:           "multi-datacenter",
			usageStats:     usageMultiDC,
			showDatacenter: true,
			expectedBillable: `
Datacenter      Services      Service instances
dc1             10            20
dc2             23            33
                              
Total           33            53`,
			expectedConnect: `
Datacenter      Type                     Service instances
dc1             connect-native           1
dc1             connect-proxy            3
dc1             ingress-gateway          4
dc1             mesh-gateway             2
dc1             terminating-gateway      5
dc2             connect-native           9
dc2             connect-proxy            8
dc2             ingress-gateway          7
dc2             mesh-gateway             6
dc2             terminating-gateway      0
                                         
Total                                    45`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			billableOutput, err := formatServiceCounts(tc.usageStats, true, tc.showDatacenter)
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(tc.expectedBillable), billableOutput)

			connectOutput, err := formatServiceCounts(tc.usageStats, false, tc.showDatacenter)
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(tc.expectedConnect), connectOutput)
		})
	}
}

func TestUsageInstances_formatNodesCounts(t *testing.T) {
	usageBasic := map[string]api.ServiceUsage{
		"dc1": {
			Nodes: 10,
		},
	}

	usageMultiDC := map[string]api.ServiceUsage{
		"dc1": {
			Nodes: 10,
		},
		"dc2": {
			Nodes: 11,
		},
	}

	cases := []struct {
		name          string
		usageStats    map[string]api.ServiceUsage
		expectedNodes string
	}{
		{
			name:       "basic",
			usageStats: usageBasic,
			expectedNodes: `
Datacenter      Count            
dc1             10
                
Total           10`,
		},
		{
			name:       "multi-datacenter",
			usageStats: usageMultiDC,
			expectedNodes: `
Datacenter      Count            
dc1             10
dc2             11
                
Total           21`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodesOutput, err := formatNodesCounts(tc.usageStats)
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(tc.expectedNodes), nodesOutput)
		})
	}
}
