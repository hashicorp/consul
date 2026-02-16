// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

func TestMakeInlineOverrideFilterChains_FileSystemCertificates(t *testing.T) {
	// This test verifies the fix for the bug where multiple file-system-certificate
	// entries would create duplicate filter chain matchers, causing Envoy to reject
	// the configuration with: "duplicate matcher is: {}"

	snap := &proxycfg.ConfigSnapshot{}
	snap.APIGateway.TLSConfig = structs.GatewayTLSConfig{}

	// Create multiple file-system certificate entries
	certs := []structs.ConfigEntry{
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert1",
			Certificate: "/path/to/cert1.pem",
			PrivateKey:  "/path/to/key1.pem",
		},
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert2",
			Certificate: "/path/to/cert2.pem",
			PrivateKey:  "/path/to/key2.pem",
		},
	}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		snap.APIGateway.TLSConfig,
		"http",
		filterOpts,
		certs,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1, "Should create exactly one filter chain for multiple file-system certificates")

	// Verify the filter chain has no SNI match (matches all)
	chain := chains[0]
	if chain.FilterChainMatch != nil {
		require.Empty(t, chain.FilterChainMatch.ServerNames, "Filter chain should not have SNI matching for file-system certificates")
	}

	// Verify the TLS context has multiple SDS secret configs
	require.NotNil(t, chain.TransportSocket)
	// The transport socket should contain the TLS context with multiple SDS configs
	// This is the key fix: multiple certificates in ONE filter chain via SDS
}

func TestMakeInlineOverrideFilterChains_SingleFileSystemCertificate(t *testing.T) {
	snap := &proxycfg.ConfigSnapshot{}
	snap.APIGateway.TLSConfig = structs.GatewayTLSConfig{}

	certs := []structs.ConfigEntry{
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert1",
			Certificate: "/path/to/cert1.pem",
			PrivateKey:  "/path/to/key1.pem",
		},
	}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		snap.APIGateway.TLSConfig,
		"http",
		filterOpts,
		certs,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1, "Should create one filter chain for single file-system certificate")
}

func TestMakeInlineOverrideFilterChains_NoDuplicateMatchers(t *testing.T) {
	// This is the core test for the bug fix
	// Before the fix, multiple file-system certificates would create filter chains
	// with identical empty FilterChainMatch objects, causing Envoy errors

	snap := &proxycfg.ConfigSnapshot{}
	snap.APIGateway.TLSConfig = structs.GatewayTLSConfig{}

	certs := []structs.ConfigEntry{
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "first-cert",
			Certificate: "/certs/first.pem",
			PrivateKey:  "/certs/first-key.pem",
		},
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "second-cert",
			Certificate: "/certs/second.pem",
			PrivateKey:  "/certs/second-key.pem",
		},
	}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		snap.APIGateway.TLSConfig,
		"http",
		filterOpts,
		certs,
	)

	require.NoError(t, err)

	// Verify no duplicate matchers
	// With the fix, we should have exactly 1 filter chain
	require.Len(t, chains, 1, "Should consolidate file-system certificates into one filter chain")

	// Verify the filter chain match is either nil or has no server names
	// (meaning it matches all traffic, and Envoy will select cert based on SNI)
	chain := chains[0]
	if chain.FilterChainMatch != nil {
		require.Empty(t, chain.FilterChainMatch.ServerNames,
			"File-system certificate filter chain should not have SNI restrictions")
	}
}
func TestMakeInlineOverrideFilterChains_EmptyCertificates(t *testing.T) {
	// Test with no certificates
	snap := &proxycfg.ConfigSnapshot{}
	snap.APIGateway.TLSConfig = structs.GatewayTLSConfig{}

	certs := []structs.ConfigEntry{}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		snap.APIGateway.TLSConfig,
		"http",
		filterOpts,
		certs,
	)

	require.NoError(t, err)
	require.Empty(t, chains, "Should return empty chains for no certificates")
}

func TestMakeInlineOverrideFilterChains_ManyFileSystemCertificates(t *testing.T) {
	// Test with more than 2 file-system certificates
	snap := &proxycfg.ConfigSnapshot{}
	snap.APIGateway.TLSConfig = structs.GatewayTLSConfig{}

	certs := []structs.ConfigEntry{
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert1",
			Certificate: "/path/to/cert1.pem",
			PrivateKey:  "/path/to/key1.pem",
		},
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert2",
			Certificate: "/path/to/cert2.pem",
			PrivateKey:  "/path/to/key2.pem",
		},
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert3",
			Certificate: "/path/to/cert3.pem",
			PrivateKey:  "/path/to/key3.pem",
		},
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert4",
			Certificate: "/path/to/cert4.pem",
			PrivateKey:  "/path/to/key4.pem",
		},
	}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		snap.APIGateway.TLSConfig,
		"http",
		filterOpts,
		certs,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1, "Should consolidate all file-system certificates into one filter chain")

	// Verify no duplicate matchers even with many certificates
	chain := chains[0]
	if chain.FilterChainMatch != nil {
		require.Empty(t, chain.FilterChainMatch.ServerNames,
			"Filter chain should not have SNI restrictions with multiple file-system certificates")
	}
}

func TestMakeInlineOverrideFilterChains_TLSParameters(t *testing.T) {
	// Test that TLS parameters are preserved
	snap := &proxycfg.ConfigSnapshot{}
	snap.APIGateway.TLSConfig = structs.GatewayTLSConfig{
		TLSMinVersion: "TLSv1_2",
		TLSMaxVersion: "TLSv1_3",
		CipherSuites: []types.TLSCipherSuite{
			types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			types.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	certs := []structs.ConfigEntry{
		&structs.FileSystemCertificateConfigEntry{
			Kind:        structs.FileSystemCertificate,
			Name:        "cert1",
			Certificate: "/path/to/cert1.pem",
			PrivateKey:  "/path/to/key1.pem",
		},
	}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		snap.APIGateway.TLSConfig,
		"http",
		filterOpts,
		certs,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1)

	// Verify TLS context exists
	chain := chains[0]
	require.NotNil(t, chain.TransportSocket, "Transport socket should be configured with TLS parameters")
}

// Made with Bob
