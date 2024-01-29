// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package discovery

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/stretchr/testify/require"
)

type testCaseParseLocality struct {
	name                string
	labels              []string
	defaultMeta         acl.EnterpriseMeta
	enterpriseDNSConfig EnterpriseDNSConfig
	expectedResult      QueryLocality
	expectedOK          bool
}

func Test_parseLocality(t *testing.T) {
	testCases := getTestCases()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, actualOK := ParseLocality(tc.labels, tc.defaultMeta, tc.enterpriseDNSConfig)
			require.Equal(t, tc.expectedOK, actualOK)
			require.Equal(t, tc.expectedResult, actualResult)

		})
	}

}

func Test_effectiveDatacenter(t *testing.T) {
	type testCase struct {
		name          string
		QueryLocality QueryLocality
		defaultDC     string
		expected      string
	}
	testCases := []testCase{
		{
			name: "return Datacenter first",
			QueryLocality: QueryLocality{
				Datacenter:       "test-dc",
				PeerOrDatacenter: "test-peer",
			},
			defaultDC: "default-dc",
			expected:  "test-dc",
		},
		{
			name: "return PeerOrDatacenter second",
			QueryLocality: QueryLocality{
				PeerOrDatacenter: "test-peer",
			},
			defaultDC: "default-dc",
			expected:  "test-peer",
		},
		{
			name:          "return defaultDC as fallback",
			QueryLocality: QueryLocality{},
			defaultDC:     "default-dc",
			expected:      "default-dc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.QueryLocality.EffectiveDatacenter(tc.defaultDC)
			require.Equal(t, tc.expected, got)
		})
	}
}
