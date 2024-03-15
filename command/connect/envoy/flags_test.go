// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package envoy

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestServiceAddressValue_Value(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var addr *ServiceAddressValue
		require.Equal(t, api.ServiceAddress{Port: defaultGatewayPort}, addr.Value())
	})

	t.Run("default value", func(t *testing.T) {
		addr := &ServiceAddressValue{}
		require.Equal(t, api.ServiceAddress{Port: defaultGatewayPort}, addr.Value())
	})

	t.Run("set value", func(t *testing.T) {
		addr := &ServiceAddressValue{}
		require.NoError(t, addr.Set("localhost:3333"))
		require.Equal(t, api.ServiceAddress{
			Address: "localhost",
			Port:    3333,
		}, addr.Value())
	})
}

func TestServiceAddressValue_String(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var addr *ServiceAddressValue
		require.Equal(t, ":8443", addr.String())
	})

	t.Run("default value", func(t *testing.T) {
		addr := &ServiceAddressValue{}
		require.Equal(t, ":8443", addr.String())
	})

	t.Run("set value", func(t *testing.T) {
		addr := &ServiceAddressValue{}
		require.NoError(t, addr.Set("localhost:3333"))
		require.Equal(t, "localhost:3333", addr.String())
	})
}

func TestServiceAddressValue_Set(t *testing.T) {
	var testcases = []struct {
		name          string
		input         string
		expectedErr   string
		expectedValue api.ServiceAddress
	}{
		{
			name:  "default port",
			input: "8.8.8.8:",
			expectedValue: api.ServiceAddress{
				Address: "8.8.8.8",
				Port:    defaultGatewayPort,
			},
		},
		{
			name:          "valid address",
			input:         "8.8.8.8:1234",
			expectedValue: api.ServiceAddress{Address: "8.8.8.8", Port: 1234},
		},
		{
			name:  "address with no port",
			input: "8.8.8.8",
			expectedValue: api.ServiceAddress{
				Address: "8.8.8.8",
				Port:    defaultGatewayPort,
			},
		},
		{
			name:        "invalid addres",
			input:       "not-an-ip-address",
			expectedErr: "not an IP address",
		},
		{
			name:        "invalid port",
			input:       "localhost:notaport",
			expectedErr: `Error parsing port "notaport"`,
		},
		{
			name:        "invalid address format",
			input:       "too:many:colons",
			expectedErr: "address too:many:colons: too many colons",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			addr := &ServiceAddressValue{}
			err := addr.Set(tc.input)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				return
			}

			require.Equal(t, tc.expectedValue, addr.Value())
		})
	}
}
