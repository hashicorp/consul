// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_parseLabels(t *testing.T) {
	type testCase struct {
		name           string
		labels         []string
		expectedOK     bool
		expectedResult *parsedLabels
	}
	testCases := []testCase{
		{
			name:   "6 labels - with datacenter",
			labels: []string{"test-ns", "ns", "test-ap", "ap", "test-dc", "dc"},
			expectedResult: &parsedLabels{
				Namespace:  "test-ns",
				Partition:  "test-ap",
				Datacenter: "test-dc",
			},
			expectedOK: true,
		},
		{
			name:   "6 labels - with cluster",
			labels: []string{"test-ns", "ns", "test-ap", "ap", "test-cluster", "cluster"},
			expectedResult: &parsedLabels{
				Namespace:  "test-ns",
				Partition:  "test-ap",
				Datacenter: "test-cluster",
			},
			expectedOK: true,
		},
		{
			name:   "6 labels - with peer",
			labels: []string{"test-ns", "ns", "test-ap", "ap", "test-peer", "peer"},
			expectedResult: &parsedLabels{
				Namespace: "test-ns",
				Partition: "test-ap",
				Peer:      "test-peer",
			},
			expectedOK: true,
		},
		{
			name:   "6 labels - with sameness group",
			labels: []string{"test-sg", "sg", "test-ap", "ap", "test-ns", "ns"},
			expectedResult: &parsedLabels{
				Namespace:     "test-ns",
				Partition:     "test-ap",
				SamenessGroup: "test-sg",
			},
			expectedOK: true,
		},
		{
			name:           "6 labels - invalid",
			labels:         []string{"test-ns", "not-ns", "test-ap", "ap", "test-dc", "dc"},
			expectedResult: nil,
			expectedOK:     false,
		},
		{
			name:   "4 labels - namespace and datacenter",
			labels: []string{"test-ns", "ns", "test-ap", "ap"},
			expectedResult: &parsedLabels{
				Namespace: "test-ns",
				Partition: "test-ap",
			},
			expectedOK: true,
		},
		{
			name:           "4 labels - invalid",
			labels:         []string{"test-ns", "not-ns", "test-ap", "ap", "test-dc", "dc"},
			expectedResult: nil,
			expectedOK:     false,
		},
		{
			name:   "2 labels - namespace and peer or datacenter",
			labels: []string{"test-ns", "test-peer-or-dc"},
			expectedResult: &parsedLabels{
				Namespace:        "test-ns",
				PeerOrDatacenter: "test-peer-or-dc",
			},
			expectedOK: true,
		},
		{
			name:   "1 label - peer or datacenter",
			labels: []string{"test-peer-or-dc"},
			expectedResult: &parsedLabels{
				PeerOrDatacenter: "test-peer-or-dc",
			},
			expectedOK: true,
		},
		{
			name:           "0 labels - returns empty result and true",
			labels:         []string{},
			expectedResult: &parsedLabels{},
			expectedOK:     true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := parseLabels(tc.labels)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

func Test_parsePort(t *testing.T) {
	type testCase struct {
		name           string
		labels         []string
		expectedResult string
	}
	testCases := []testCase{
		{
			name:           "given 3 labels where the second label is port, the first label is returned",
			labels:         []string{"port-name", "port", "target-name"},
			expectedResult: "port-name",
		},
		{
			name:           "given 3 labels where the second label is not port, an empty string is returned",
			labels:         []string{"port-name", "not-port", "target-name"},
			expectedResult: "",
		},
		{
			name:           "given anything but 3 labels, an empty string is returned",
			labels:         []string{"port-name", "something-else"},
			expectedResult: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedResult, parsePort(tc.labels))
		})
	}
}
