// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupTLSConfig(t *testing.T) {
	t.Run("verifies the server certificate by default", func(t *testing.T) {
		cfg, err := SetupTLSConfig(&GRPCConfig{Address: "consul.example.com:8503", GRPCTLS: true})
		require.NoError(t, err)
		require.False(t, cfg.InsecureSkipVerify)
		require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
		// An empty ServerName makes gRPC verify against the host in the dial address.
		require.Empty(t, cfg.ServerName)
	})

	t.Run("verification can be explicitly disabled", func(t *testing.T) {
		cfg, err := SetupTLSConfig(&GRPCConfig{GRPCTLS: true, InsecureSkipVerify: true})
		require.NoError(t, err)
		require.True(t, cfg.InsecureSkipVerify)
		require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	})

	t.Run("server name", func(t *testing.T) {
		cfg, err := SetupTLSConfig(&GRPCConfig{GRPCTLS: true, TLSServerName: "server.dc1.consul"})
		require.NoError(t, err)
		require.Equal(t, "server.dc1.consul", cfg.ServerName)
	})

	t.Run("server name forms", func(t *testing.T) {
		cases := map[string]string{
			"server.dc1.consul":      "server.dc1.consul",
			"server.dc1.consul:8503": "server.dc1.consul",
			"10.0.0.5":               "10.0.0.5",
			"10.0.0.5:8503":          "10.0.0.5",
			"2001:db8::1":            "2001:db8::1",
			"[2001:db8::1]":          "2001:db8::1",
			"[2001:db8::1]:8503":     "2001:db8::1",
			"::1":                    "::1",
			"[::1]":                  "::1",
			"[::1]:8502":             "::1",
		}
		for in, expect := range cases {
			t.Run(in, func(t *testing.T) {
				cfg, err := SetupTLSConfig(&GRPCConfig{GRPCTLS: true, TLSServerName: in})
				require.NoError(t, err)
				require.Equal(t, expect, cfg.ServerName)
			})
		}
	})

	t.Run("CA file", func(t *testing.T) {
		cfg, err := SetupTLSConfig(&GRPCConfig{GRPCTLS: true, CAFile: "../../../test/client_certs/rootca.crt"})
		require.NoError(t, err)
		require.NotNil(t, cfg.RootCAs)
	})

	t.Run("invalid CA file", func(t *testing.T) {
		_, err := SetupTLSConfig(&GRPCConfig{GRPCTLS: true, CAFile: "../../../test/client_certs/does-not-exist.crt"})
		require.Error(t, err)
	})

	t.Run("client certificate", func(t *testing.T) {
		cfg, err := SetupTLSConfig(&GRPCConfig{
			GRPCTLS:  true,
			CertFile: "../../../test/client_certs/client.crt",
			KeyFile:  "../../../test/client_certs/client.key",
		})
		require.NoError(t, err)
		require.Len(t, cfg.Certificates, 1)
	})

	t.Run("client certificate without key", func(t *testing.T) {
		_, err := SetupTLSConfig(&GRPCConfig{GRPCTLS: true, CertFile: "../../../test/client_certs/client.crt"})
		require.ErrorContains(t, err, "both client cert and client key must be provided")
	})

	t.Run("client key without certificate", func(t *testing.T) {
		_, err := SetupTLSConfig(&GRPCConfig{GRPCTLS: true, KeyFile: "../../../test/client_certs/client.key"})
		require.ErrorContains(t, err, "both client cert and client key must be provided")
	})
}

// TestNewGRPCClient_IncompleteClientCertificate checks the CLI path. A
// certificate without a key, or a key without a certificate, has always been
// rejected by checkCertificates before SetupTLSConfig runs, so the CLI's
// behavior and error message are unchanged.
func TestNewGRPCClient_IncompleteClientCertificate(t *testing.T) {
	for name, c := range map[string]*GRPCConfig{
		"certificate without key": {Address: "127.0.0.1:1", GRPCTLS: true, CertFile: "../../../test/client_certs/client.crt"},
		"key without certificate": {Address: "127.0.0.1:1", GRPCTLS: true, KeyFile: "../../../test/client_certs/client.key"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewGRPCClient(c)
			require.ErrorContains(t, err, "you have to provide client certificate file and key file at the same time")
		})
	}
}

func TestTValue(t *testing.T) {
	t.Run("String: set", func(t *testing.T) {
		var tv TValue[string]

		err := tv.Set("testString")
		assert.NoError(t, err)

		assert.Equal(t, *tv.v, "testString")
	})

	t.Run("String: merge", func(t *testing.T) {
		var tv TValue[string]
		var onto string

		testStr := "testString"
		tv.v = &testStr
		tv.Merge(&onto)

		assert.Equal(t, onto, "testString")
	})

	t.Run("String: merge nil", func(t *testing.T) {
		var tv TValue[string]
		var onto *string = nil

		testStr := "testString"
		tv.v = &testStr
		err := tv.Merge(onto)

		assert.Equal(t, err.Error(), "onto is nil")
	})

	t.Run("Get string", func(t *testing.T) {
		var tv TValue[string]
		testStr := "testString"
		tv.v = &testStr
		assert.Equal(t, tv.String(), "testString")
	})

	t.Run("Bool: set", func(t *testing.T) {
		var tv TValue[bool]

		err := tv.Set("true")
		assert.NoError(t, err)

		assert.Equal(t, *tv.v, true)
	})

	t.Run("Bool: merge", func(t *testing.T) {
		var tv TValue[bool]
		var onto bool

		testBool := true
		tv.v = &testBool
		tv.Merge(&onto)

		assert.Equal(t, onto, true)
	})
}
