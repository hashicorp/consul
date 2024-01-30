// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"testing"

	"github.com/miekg/dns"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCaseBuildQueryFromDNSMessage is a test case for the buildQueryFromDNSMessage function.
type testCaseBuildQueryFromDNSMessage struct {
	name           string
	request        *dns.Msg
	requestContext *discovery.Context
	expectedQuery  *discovery.Query
}

// Test_buildQueryFromDNSMessage tests the buildQueryFromDNSMessage function.
func Test_buildQueryFromDNSMessage(t *testing.T) {

	testCases := getBuildQueryFromDNSMessageTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := buildQueryFromDNSMessage(tc.request, "domain", "altDomain", &RouterDynamicConfig{}, acl.EnterpriseMeta{}, "defaultDatacenter")
			require.NoError(t, err)
			assert.Equal(t, tc.expectedQuery, query)
		})
	}
}
