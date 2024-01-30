// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package discovery

import (
	"github.com/hashicorp/consul/acl"
)

func getTestCases() []testCaseParseLocality {
	testCases := []testCaseParseLocality{
		{
			name:                "test [.<datacenter>.dc]",
			labels:              []string{"test-dc", "dc"},
			enterpriseDNSConfig: EnterpriseDNSConfig{},
			expectedResult: QueryLocality{
				EnterpriseMeta: acl.EnterpriseMeta{},
				Datacenter:     "test-dc",
			},
			expectedOK: true,
		},
		{
			name:                "test [.<peer>.peer]",
			labels:              []string{"test-peer", "peer"},
			enterpriseDNSConfig: EnterpriseDNSConfig{},
			expectedResult: QueryLocality{
				EnterpriseMeta: acl.EnterpriseMeta{},
				Peer:           "test-peer",
			},
			expectedOK: true,
		},
		{
			name:                "test 1 label",
			labels:              []string{"test-peer"},
			enterpriseDNSConfig: EnterpriseDNSConfig{},
			expectedResult: QueryLocality{
				EnterpriseMeta:   acl.EnterpriseMeta{},
				PeerOrDatacenter: "test-peer",
			},
			expectedOK: true,
		},
		{
			name:                "test 0 labels",
			labels:              []string{},
			enterpriseDNSConfig: EnterpriseDNSConfig{},
			expectedResult:      QueryLocality{},
			expectedOK:          true,
		},
		{
			name:                "test 3 labels returns not found",
			labels:              []string{"test-dc", "dc", "test-blah"},
			enterpriseDNSConfig: EnterpriseDNSConfig{},
			expectedResult:      QueryLocality{},
			expectedOK:          false,
		},
	}
	return testCases
}
