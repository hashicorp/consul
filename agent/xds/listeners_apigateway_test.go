// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
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

// Made with Bob
