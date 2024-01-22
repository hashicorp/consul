// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package dns

import (
	"github.com/hashicorp/consul/acl"
)

func getTestCases() []testCaseParseLocality {
	testCases := []testCaseParseLocality{
		{
			name:                "test [.<datacenter>.dc]",
			labels:              []string{"test-dc", "dc"},
			enterpriseDNSConfig: enterpriseDNSConfig{},
			expectedResult: queryLocality{
				EnterpriseMeta: acl.EnterpriseMeta{},
				datacenter:     "test-dc",
			},
			expectedOK: true,
		},
		{
			name:                "test [.<peer>.peer]",
			labels:              []string{"test-peer", "peer"},
			enterpriseDNSConfig: enterpriseDNSConfig{},
			expectedResult: queryLocality{
				EnterpriseMeta: acl.EnterpriseMeta{},
				peer:           "test-peer",
			},
			expectedOK: true,
		},
		{
			name:                "test 1 label",
			labels:              []string{"test-peer"},
			enterpriseDNSConfig: enterpriseDNSConfig{},
			expectedResult: queryLocality{
				EnterpriseMeta:   acl.EnterpriseMeta{},
				peerOrDatacenter: "test-peer",
			},
			expectedOK: true,
		},
		{
			name:                "test 0 labels",
			labels:              []string{},
			enterpriseDNSConfig: enterpriseDNSConfig{},
			expectedResult:      queryLocality{},
			expectedOK:          true,
		},
		{
			name:                "test 3 labels returns not found",
			labels:              []string{"test-dc", "dc", "test-blah"},
			enterpriseDNSConfig: enterpriseDNSConfig{},
			expectedResult:      queryLocality{},
			expectedOK:          false,
		},
	}
	return testCases
}
