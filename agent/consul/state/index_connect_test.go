// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestConnectNameFromServiceNode(t *testing.T) {
	cases := []struct {
		name       string
		input      structs.ServiceNode
		expected   string
		expectedOk bool
	}{
		{
			name:       "typical service, not native",
			input:      structs.ServiceNode{ServiceName: "db"},
			expectedOk: false,
		},

		{
			name: "typical service, is native",
			input: structs.ServiceNode{
				ServiceName:    "dB",
				ServiceConnect: structs.ServiceConnect{Native: true},
			},
			expectedOk: true,
			expected:   "dB",
		},
		{
			name: "proxy service",
			input: structs.ServiceNode{
				ServiceKind:  structs.ServiceKindConnectProxy,
				ServiceName:  "db",
				ServiceProxy: structs.ConnectProxyConfig{DestinationServiceName: "fOo"},
			},
			expectedOk: true,
			expected:   "fOo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual, ok := connectNameFromServiceNode(&tc.input)
			if !tc.expectedOk {
				require.False(t, ok, "expected no connect name")
				return
			}
			require.Equal(t, tc.expected, actual)
		})
	}
}
