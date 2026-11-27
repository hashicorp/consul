// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestDefaultConfig_SSLVerifyEnv(t *testing.T) {
	cases := []struct {
		value          string
		expectInsecure bool
	}{
		{value: "true", expectInsecure: false},
		{value: "1", expectInsecure: false},
		{value: "false", expectInsecure: true},
		{value: "0", expectInsecure: true},
		// Values that cannot be parsed must not disable verification.
		{value: "yes", expectInsecure: false},
		{value: "on", expectInsecure: false},
		{value: "no", expectInsecure: false},
		{value: "ture", expectInsecure: false},
		{value: "TRUE ", expectInsecure: false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv(HTTPSSLVerifyEnvName, tc.value)

			config := DefaultConfigWithLogger(testutil.Logger(t))
			require.Equal(t, tc.expectInsecure, config.TLSConfig.InsecureSkipVerify)
		})
	}

	t.Run("unset", func(t *testing.T) {
		t.Setenv(HTTPSSLVerifyEnvName, "")
		require.False(t, DefaultConfig().TLSConfig.InsecureSkipVerify)
	})
}

func TestDefaultConfig_SSLEnv(t *testing.T) {
	cases := []struct {
		value        string
		expectScheme string
	}{
		{value: "true", expectScheme: "https"},
		{value: "1", expectScheme: "https"},
		{value: "false", expectScheme: "http"},
		{value: "0", expectScheme: "http"},
		// Values that cannot be parsed must not fall back to plain HTTP.
		{value: "yes", expectScheme: "https"},
		{value: "no", expectScheme: "https"},
		{value: "ture", expectScheme: "https"},
		{value: "TRUE ", expectScheme: "https"},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv(HTTPSSLEnvName, tc.value)

			config := DefaultConfigWithLogger(testutil.Logger(t))
			require.Equal(t, tc.expectScheme, config.Scheme)
		})
	}

	t.Run("unset", func(t *testing.T) {
		t.Setenv(HTTPSSLEnvName, "")
		require.Equal(t, "http", DefaultConfig().Scheme)
	})
}
