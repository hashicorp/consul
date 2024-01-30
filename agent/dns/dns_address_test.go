// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package dns

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_dnsAddress(t *testing.T) {
	const domain = "consul."
	type expectedResults struct {
		isIp               bool
		stringResult       string
		fqdn               string
		isFQDN             bool
		isEmptyString      bool
		isExternalFQDN     bool
		isInternalFQDN     bool
		isInternalFQDNOrIP bool
	}
	type testCase struct {
		name            string
		input           string
		expectedResults expectedResults
	}
	testCases := []testCase{
		{
			name:  "empty string",
			input: "",
			expectedResults: expectedResults{
				isIp:               false,
				stringResult:       "",
				fqdn:               "",
				isFQDN:             false,
				isEmptyString:      true,
				isExternalFQDN:     false,
				isInternalFQDN:     false,
				isInternalFQDNOrIP: false,
			},
		},
		{
			name:  "ipv4 address",
			input: "127.0.0.1",
			expectedResults: expectedResults{
				isIp:               true,
				stringResult:       "127.0.0.1",
				fqdn:               "",
				isFQDN:             false,
				isEmptyString:      false,
				isExternalFQDN:     false,
				isInternalFQDN:     false,
				isInternalFQDNOrIP: true,
			},
		},
		{
			name:  "ipv6 address",
			input: "2001:db8:1:2:cafe::1337",
			expectedResults: expectedResults{
				isIp:               true,
				stringResult:       "2001:db8:1:2:cafe::1337",
				fqdn:               "",
				isFQDN:             false,
				isEmptyString:      false,
				isExternalFQDN:     false,
				isInternalFQDN:     false,
				isInternalFQDNOrIP: true,
			},
		},
		{
			name:  "internal FQDN without trailing period",
			input: "web.service.consul",
			expectedResults: expectedResults{
				isIp:               false,
				stringResult:       "web.service.consul",
				fqdn:               "web.service.consul.",
				isFQDN:             true,
				isEmptyString:      false,
				isExternalFQDN:     false,
				isInternalFQDN:     true,
				isInternalFQDNOrIP: true,
			},
		},
		{
			name:  "internal FQDN with period",
			input: "web.service.consul.",
			expectedResults: expectedResults{
				isIp:               false,
				stringResult:       "web.service.consul.",
				fqdn:               "web.service.consul.",
				isFQDN:             true,
				isEmptyString:      false,
				isExternalFQDN:     false,
				isInternalFQDN:     true,
				isInternalFQDNOrIP: true,
			},
		},
		{
			name:  "external FQDN without trailing period",
			input: "web.service.vault",
			expectedResults: expectedResults{
				isIp:               false,
				stringResult:       "web.service.vault",
				fqdn:               "web.service.vault.",
				isFQDN:             true,
				isEmptyString:      false,
				isExternalFQDN:     true,
				isInternalFQDN:     false,
				isInternalFQDNOrIP: false,
			},
		},
		{
			name:  "external FQDN with trailing period",
			input: "web.service.vault.",
			expectedResults: expectedResults{
				isIp:               false,
				stringResult:       "web.service.vault.",
				fqdn:               "web.service.vault.",
				isFQDN:             true,
				isEmptyString:      false,
				isExternalFQDN:     true,
				isInternalFQDN:     false,
				isInternalFQDNOrIP: false,
			},
		},
		{
			name:  "another external FQDN",
			input: "www.google.com",
			expectedResults: expectedResults{
				isIp:               false,
				stringResult:       "www.google.com",
				fqdn:               "www.google.com.",
				isFQDN:             true,
				isEmptyString:      false,
				isExternalFQDN:     true,
				isInternalFQDN:     false,
				isInternalFQDNOrIP: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dnsAddress := newDNSAddress(tc.input)
			assert.Equal(t, tc.expectedResults.isIp, dnsAddress.IsIP())
			assert.Equal(t, tc.expectedResults.stringResult, dnsAddress.String())
			assert.Equal(t, tc.expectedResults.isFQDN, dnsAddress.IsFQDN())
			assert.Equal(t, tc.expectedResults.isEmptyString, dnsAddress.IsEmptyString())
			assert.Equal(t, tc.expectedResults.isExternalFQDN, dnsAddress.IsExternalFQDN(domain))
			assert.Equal(t, tc.expectedResults.isInternalFQDN, dnsAddress.IsInternalFQDN(domain))
			assert.Equal(t, tc.expectedResults.isInternalFQDNOrIP, dnsAddress.IsInternalFQDNOrIP(domain))
		})
	}
}
