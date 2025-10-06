// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestAgentRetryNewDiscover(t *testing.T) {
	d, err := newDiscover()
	require.NoError(t, err)
	expected := []string{
		"aliyun", "aws", "azure", "digitalocean", "gce", "hcp", "k8s", "linode",
		"mdns", "os", "packet", "scaleway", "softlayer", "tencentcloud",
		"triton", "vsphere",
	}
	require.Equal(t, expected, d.Names())
}

func TestAgentRetryJoinAddrs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	d, err := newDiscover()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
		dnsCache *dnsCache
	}{
		{"handles nil", nil, []string{}, newDNSCache(10 * time.Second)},
		{"handles empty input", []string{}, []string{}, newDNSCache(10 * time.Second)},
		{"handles one element",
			[]string{"192.168.0.12"},
			[]string{"192.168.0.12"},
			newDNSCache(10 * time.Second),
		},
		{"handles two elements",
			[]string{"192.168.0.12", "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
			newDNSCache(10 * time.Second),
		},
		{"tries to resolve aws things, which fails but that is fine",
			[]string{"192.168.0.12", "provider=aws region=eu-west-1 tag_key=consul tag_value=tag access_key_id=a secret_access_key=a"},
			[]string{"192.168.0.12"},
			newDNSCache(10 * time.Second),
		},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := testutil.LoggerWithOutput(t, &buf)

			output := retryJoinAddrs(d, retryJoinSerfVariant, "LAN", test.input, test.dnsCache, logger)
			bufout := buf.String()
			require.Equal(t, test.expected, output, bufout)
			if i == 4 {
				require.Contains(t, bufout, `Using provider "aws"`)
			}
		})
	}
	t.Run("handles nil discover", func(t *testing.T) {
		require.Equal(t, []string{}, retryJoinAddrs(nil, retryJoinSerfVariant, "LAN", []string{"a"}, nil, nil))
	})
}

func TestAgentRetryJoinAddrsWithDNSCache(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	d, err := newDiscover()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
		dnsCache *dnsCache
	}{
		{
			name:     "handles nil",
			input:    nil,
			expected: []string{},
			dnsCache: newDNSCache(10 * time.Second),
		},
		{
			name:     "handles empty input",
			input:    []string{},
			expected: []string{},
			dnsCache: newDNSCache(10 * time.Second),
		},
		{
			name:     "handles one element",
			input:    []string{"192.168.0.12"},
			expected: []string{"192.168.0.12"},
			dnsCache: newDNSCache(10 * time.Second),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := testutil.LoggerWithOutput(t, &buf)

			output := retryJoinAddrs(d, retryJoinSerfVariant, "LAN", test.input, test.dnsCache, logger)
			bufout := buf.String()
			require.Equal(t, test.expected, output, bufout)
		})
	}
}

func TestDNSCache(t *testing.T) {
	cache := newDNSCache(500 * time.Millisecond)

	// Test setting and getting
	cache.set("test-host", []string{"192.168.0.12"})
	addrs, expires, found := cache.get("test-host")
	require.True(t, found)
	require.Equal(t, []string{"192.168.0.12"}, addrs)
	require.True(t, expires.After(time.Now()))

	// Test expiration
	cache.entries["test-host"].expires = time.Now().Add(-1 * time.Hour)
	_, _, found = cache.get("test-host")
	require.False(t, found)
}

func TestRetryJoin(t *testing.T) {
	// Mock DNS resolution to avoid real network calls
	originalResolver := resolveHostnameFunc
	defer func() { resolveHostnameFunc = originalResolver }()
	hostNameMap := map[string]string{
		"test-host-1": "192.168.1.1",
		"test-host-2": "192.168.1.2",
	}
	resolveHostnameFunc = func(addr string) ([]string, error) {
		// Return fake IPs for our test hostnames
		ip, ok := hostNameMap[addr]
		if ok {
			return []string{ip}, nil
		}

		return nil, fmt.Errorf("unknown hostname: %s", addr)
	}
	tests := []struct {
		name        string
		join        func(addrs []string) (int, error)
		addrs       []string
		shouldFind  bool
		shouldError bool
		description string
	}{
		{
			name: "should pass",
			join: func(addrs []string) (int, error) {
				return 0, nil
			},
			addrs:       []string{"test-host-1"},
			shouldFind:  true,
			shouldError: false,
			description: "successful join should keep cache entry",
		},
		{
			name: "connection refused",
			join: func(addrs []string) (int, error) {
				return 0, fmt.Errorf("dial tcp 192.168.1.2:8301: connect: connection refused")
			},
			addrs:       []string{"test-host-1"},
			shouldFind:  false,
			shouldError: true,
			description: "connection refused should invalidate cache for hostnames",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockJoin := test.join
			cache := newDNSCache(1 * time.Hour)

			// Set up cache based on test case
			for _, host := range test.addrs {
				cache.set(host, []string{"192.168.1.1"})
			}

			// Verify initial cache state for hostname tests
			for _, host := range test.addrs {
				_, _, found := cache.get(host)
				require.True(t, found, "%s should be in cache initially", host)
			}

			r := &retryJoiner{
				variant:     retryJoinSerfVariant,
				cluster:     "LAN",
				addrs:       test.addrs,
				maxAttempts: 1,
				interval:    time.Millisecond,
				join:        mockJoin,
				logger:      testutil.Logger(t),
				dnsCache:    cache,
			}
			err := r.retryJoin()
			if test.shouldError {
				require.Error(t, err, "Should fail")
			} else {
				require.NoError(t, err, "Should succeed")
			}

			// Check cache state based on test case
			for _, host := range test.addrs {
				_, _, found := cache.get(host)
				require.Equal(t, test.shouldFind, found, "%s: cache entry should be %v but is %v",
					test.description, test.shouldFind, found)
			}
		})
	}
}
