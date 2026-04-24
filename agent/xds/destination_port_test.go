// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

import "testing"

func TestDestinationPortClusterName(t *testing.T) {
	cases := map[string]struct {
		clusterName     string
		destinationPort string
		expected        string
	}{
		"empty destination port": {
			clusterName:     "api-app.default.dc1.consul",
			destinationPort: "",
			expected:        "api-app.default.dc1.consul",
		},
		"destination port present": {
			clusterName:     "api-app.default.dc1.consul",
			destinationPort: "admin-port",
			expected:        "api-app.default.dc1.consul",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := destinationPortClusterName(tc.clusterName, tc.destinationPort)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestDestinationPortALPN(t *testing.T) {
	cases := map[string]struct {
		destinationPort string
		expectedLen     int
		expectedValue   string
	}{
		"empty destination port": {
			destinationPort: "",
			expectedLen:     0,
		},
		"destination port present": {
			destinationPort: "api-port",
			expectedLen:     0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := destinationPortALPN(tc.destinationPort)
			if len(got) != tc.expectedLen {
				t.Fatalf("expected length %d, got %d", tc.expectedLen, len(got))
			}
			if tc.expectedLen > 0 && got[0] != tc.expectedValue {
				t.Fatalf("expected %q, got %q", tc.expectedValue, got[0])
			}
		})
	}
}

func TestDestinationPortListenerName(t *testing.T) {
	cases := map[string]struct {
		baseName        string
		destinationPort string
		expected        string
	}{
		"empty destination port": {
			baseName:        "responseService",
			destinationPort: "",
			expected:        "responseService",
		},
		"destination port with query-style upstream id": {
			baseName:        "responseService?port=admin-port",
			destinationPort: "admin-port",
			expected:        "responseService",
		},
		"destination port with dc query": {
			baseName:        "responseService?dc=dc2&port=admin-port",
			destinationPort: "admin-port",
			expected:        "responseService",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := destinationPortListenerName(tc.baseName, tc.destinationPort)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
