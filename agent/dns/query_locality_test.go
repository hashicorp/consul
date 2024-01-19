// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/stretchr/testify/require"
)

type testCaseParseLocality struct {
	name                string
	labels              []string
	defaultMeta         acl.EnterpriseMeta
	enterpriseDNSConfig enterpriseDNSConfig
	expectedResult      queryLocality
	expectedOK          bool
}

func Test_ParseLocality(t *testing.T) {
	testCases := getTestCases()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, actualOK := ParseLocality(tc.labels, tc.defaultMeta, tc.enterpriseDNSConfig)
			require.Equal(t, tc.expectedOK, actualOK)
			require.Equal(t, tc.expectedResult, actualResult)

		})
	}

}

func Test_EffectiveDatacenter(t *testing.T) {
	type testCase struct {
		name          string
		queryLocality queryLocality
		defaultDC     string
		expected      string
	}
	testCases := []testCase{
		{
			name: "return datacenter first",
			queryLocality: queryLocality{
				datacenter:       "test-dc",
				peerOrDatacenter: "test-peer",
			},
			defaultDC: "default-dc",
			expected:  "test-dc",
		},
		{
			name: "return peerOrDatacenter second",
			queryLocality: queryLocality{
				peerOrDatacenter: "test-peer",
			},
			defaultDC: "default-dc",
			expected:  "test-peer",
		},
		{
			name:          "return defaultDC as fallback",
			queryLocality: queryLocality{},
			defaultDC:     "default-dc",
			expected:      "default-dc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.queryLocality.EffectiveDatacenter(tc.defaultDC)
			require.Equal(t, tc.expected, got)
		})
	}
}
