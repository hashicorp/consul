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
		nil,
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
		nil,
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
		nil,
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

func TestMakeInlineOverrideFilterChains_TCPServiceSDSCatchAllSupersedesCertificateChains(t *testing.T) {
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

	overrides := []apiGatewayServiceSDSOverride{{
		SDS: structs.GatewayTLSSDSConfig{
			ClusterName:  "sds-cluster",
			CertResource: "service-cert",
		},
	}}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "tcp",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		snap.APIGateway.TLSConfig,
		overrides,
		"tcp",
		filterOpts,
		certs,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1, "catch-all service SDS override should suppress listener cert chains")
	require.Nil(t, chains[0].FilterChainMatch, "catch-all override chain should be the only catch-all matcher")
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
		nil,
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
		nil,
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
		nil,
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

func TestMakeInlineOverrideFilterChains_SDSCertificate(t *testing.T) {
	snap := &proxycfg.ConfigSnapshot{}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	tlsCfg := structs.GatewayTLSConfig{
		SDS: &structs.GatewayTLSSDSConfig{
			ClusterName:  "sds-cluster",
			CertResource: "api-gw-cert",
		},
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		tlsCfg,
		nil,
		"http",
		filterOpts,
		nil,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1, "SDS-configured listener should build a TLS filter chain without certificate config entries")
	require.NotNil(t, chains[0].TransportSocket)
}

func TestMakeInlineOverrideFilterChains_ServiceSDSOverrides(t *testing.T) {
	snap := &proxycfg.ConfigSnapshot{}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	overrides := []apiGatewayServiceSDSOverride{
		{
			Hosts: []string{"api.example.com"},
			SDS: structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "api-cert",
			},
		},
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		structs.GatewayTLSConfig{},
		overrides,
		"http",
		filterOpts,
		nil,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1)
	require.NotNil(t, chains[0].FilterChainMatch)
	require.ElementsMatch(t, []string{"api.example.com"}, chains[0].FilterChainMatch.ServerNames)
}

func TestMakeInlineOverrideFilterChains_ServiceSDSOverrideAndListenerDefault(t *testing.T) {
	snap := &proxycfg.ConfigSnapshot{}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	overrides := []apiGatewayServiceSDSOverride{{
		Hosts: []string{"api.example.com"},
		SDS: structs.GatewayTLSSDSConfig{
			ClusterName:  "sds-cluster",
			CertResource: "api-cert",
		},
	}}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "default-cert",
			},
		},
		overrides,
		"http",
		filterOpts,
		nil,
	)

	require.NoError(t, err)
	require.Len(t, chains, 2)
	require.NotNil(t, chains[0].FilterChainMatch)
	require.ElementsMatch(t, []string{"api.example.com"}, chains[0].FilterChainMatch.ServerNames)
	require.True(t, chains[1].FilterChainMatch == nil || len(chains[1].FilterChainMatch.ServerNames) == 0,
		"listener default SDS chain should be appended after service override chains")

	overrideCount := 0
	defaultCount := 0
	for _, chain := range chains {
		if chain.FilterChainMatch == nil || len(chain.FilterChainMatch.ServerNames) == 0 {
			defaultCount++
			continue
		}
		if len(chain.FilterChainMatch.ServerNames) == 1 && chain.FilterChainMatch.ServerNames[0] == "api.example.com" {
			overrideCount++
		}
	}
	require.Equal(t, 1, overrideCount, "expected one SNI override filter chain")
	require.Equal(t, 1, defaultCount, "expected one catch-all listener SDS filter chain")
}

func TestMakeInlineOverrideFilterChains_MultipleServiceSDSOverrides_OrderBeforeDefault(t *testing.T) {
	snap := &proxycfg.ConfigSnapshot{}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "http",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	overrides := []apiGatewayServiceSDSOverride{
		{
			Hosts: []string{"api.example.com"},
			SDS: structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "api-cert",
			},
		},
		{
			Hosts: []string{"admin.example.com"},
			SDS: structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "admin-cert",
			},
		},
	}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "default-cert",
			},
		},
		overrides,
		"http",
		filterOpts,
		nil,
	)

	require.NoError(t, err)
	require.Len(t, chains, 3)
	require.ElementsMatch(t, []string{"api.example.com"}, chains[0].FilterChainMatch.ServerNames)
	require.ElementsMatch(t, []string{"admin.example.com"}, chains[1].FilterChainMatch.ServerNames)
	require.True(t, chains[2].FilterChainMatch == nil || len(chains[2].FilterChainMatch.ServerNames) == 0,
		"listener default SDS chain should come after all override chains")
}

func TestMakeInlineOverrideFilterChains_CatchAllServiceOverrideSkipsListenerDefaultSDS(t *testing.T) {
	snap := &proxycfg.ConfigSnapshot{}

	s := ResourceGenerator{}
	filterOpts := listenerFilterOpts{
		protocol:   "tcp",
		routeName:  "test-route",
		cluster:    "test-cluster",
		statPrefix: "test",
	}

	overrides := []apiGatewayServiceSDSOverride{{
		SDS: structs.GatewayTLSSDSConfig{
			ClusterName:  "sds-cluster",
			CertResource: "service-cert",
		},
	}}

	chains, err := s.makeInlineOverrideFilterChains(
		snap,
		structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "listener-default-cert",
			},
		},
		overrides,
		"tcp",
		filterOpts,
		nil,
	)

	require.NoError(t, err)
	require.Len(t, chains, 1, "catch-all service override should suppress listener default SDS chain")
	require.True(t, chains[0].FilterChainMatch == nil || len(chains[0].FilterChainMatch.ServerNames) == 0)
}

func TestCollectAPIGatewayServiceSDSOverrides_TCPRouteInheritsListenerSDSCluster(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
		entry.Listeners = []structs.APIGatewayListener{{
			Name:     "tcp-listener",
			Protocol: structs.ListenerProtocolTCP,
			Port:     9000,
			TLS: structs.APIGatewayTLSConfiguration{
				SDS: &structs.GatewayTLSSDSConfig{ClusterName: "listener-sds-cluster", CertResource: "listener-default-cert"},
			},
		}}
		bound.Listeners = []structs.BoundAPIGatewayListener{{
			Name: "tcp-listener",
			Routes: []structs.ResourceReference{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
			}},
		}}
	}, []structs.BoundRoute{
		&structs.TCPRouteConfigEntry{
			Kind: structs.TCPRoute,
			Name: "tcp-route",
			Parents: []structs.ResourceReference{{
				Kind: structs.APIGateway,
				Name: "api-gateway",
			}},
			Services: []structs.TCPService{{
				Name: "backend",
				TLS:  &structs.GatewayServiceTLSConfig{SDS: &structs.GatewayTLSSDSConfig{CertResource: "service-cert"}},
			}},
		},
	}, nil, nil)

	listenerCfg := structs.APIGatewayListener{
		Name:     "tcp-listener",
		Protocol: structs.ListenerProtocolTCP,
		TLS: structs.APIGatewayTLSConfiguration{
			SDS: &structs.GatewayTLSSDSConfig{ClusterName: "listener-sds-cluster", CertResource: "listener-default-cert"},
		},
	}
	resolvedTLSCfg, err := resolveAPIListenerTLSConfig(snap.APIGateway.TLSConfig, listenerCfg.TLS)
	require.NoError(t, err)

	overrides, err := collectAPIGatewayServiceSDSOverridesWithResolvedTLS(snap, readyListener{
		listenerCfg: listenerCfg,
		routeReferences: map[structs.ResourceReference]struct{}{
			{Kind: structs.TCPRoute, Name: "tcp-route"}: {},
		},
	}, resolvedTLSCfg)

	require.NoError(t, err)
	require.Len(t, overrides, 1)
	require.Equal(t, "listener-sds-cluster", overrides[0].SDS.ClusterName)
	require.Equal(t, "service-cert", overrides[0].SDS.CertResource)
	require.Empty(t, overrides[0].Hosts)
}

func TestCollectAPIGatewayServiceSDSOverrides_TCPRouteRequiresClusterWhenNoListenerDefault(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
		entry.Listeners = []structs.APIGatewayListener{{
			Name:     "tcp-listener",
			Protocol: structs.ListenerProtocolTCP,
			Port:     9000,
		}}
		bound.Listeners = []structs.BoundAPIGatewayListener{{
			Name: "tcp-listener",
			Routes: []structs.ResourceReference{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
			}},
		}}
	}, []structs.BoundRoute{
		&structs.TCPRouteConfigEntry{
			Kind: structs.TCPRoute,
			Name: "tcp-route",
			Parents: []structs.ResourceReference{{
				Kind: structs.APIGateway,
				Name: "api-gateway",
			}},
			Services: []structs.TCPService{{
				Name: "backend",
				TLS:  &structs.GatewayServiceTLSConfig{SDS: &structs.GatewayTLSSDSConfig{CertResource: "service-cert"}},
			}},
		},
	}, nil, nil)

	listenerCfg := structs.APIGatewayListener{
		Name:     "tcp-listener",
		Protocol: structs.ListenerProtocolTCP,
	}
	resolvedTLSCfg, err := resolveAPIListenerTLSConfig(snap.APIGateway.TLSConfig, listenerCfg.TLS)
	require.NoError(t, err)

	_, err = collectAPIGatewayServiceSDSOverridesWithResolvedTLS(snap, readyListener{
		listenerCfg: listenerCfg,
		routeReferences: map[structs.ResourceReference]struct{}{
			{Kind: structs.TCPRoute, Name: "tcp-route"}: {},
		},
	}, resolvedTLSCfg)

	require.Error(t, err)
	require.Contains(t, err.Error(), "sets TLS.SDS without ClusterName")
}

func TestCollectAPIGatewayServiceSDSOverrides_TCPRouteRejectsConflictingOverrides(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
		entry.Listeners = []structs.APIGatewayListener{{
			Name:     "tcp-listener",
			Protocol: structs.ListenerProtocolTCP,
			Port:     9000,
		}}
		bound.Listeners = []structs.BoundAPIGatewayListener{{
			Name: "tcp-listener",
			Routes: []structs.ResourceReference{
				{Kind: structs.TCPRoute, Name: "tcp-route-1"},
			},
		}}
	}, []structs.BoundRoute{
		&structs.TCPRouteConfigEntry{
			Kind: structs.TCPRoute,
			Name: "tcp-route-1",
			Parents: []structs.ResourceReference{{
				Kind: structs.APIGateway,
				Name: "api-gateway",
			}},
			Services: []structs.TCPService{{
				Name: "backend-a",
				TLS:  &structs.GatewayServiceTLSConfig{SDS: &structs.GatewayTLSSDSConfig{ClusterName: "sds-cluster", CertResource: "service-a-cert"}},
			}},
		},
	}, nil, nil)
	if snap == nil {
		t.Fatal("expected non-nil config snapshot")
	}

	routeRef1 := structs.ResourceReference{Kind: structs.TCPRoute, Name: "tcp-route-1"}
	routeRef2 := structs.ResourceReference{Kind: structs.TCPRoute, Name: "tcp-route-2"}

	snap.APIGateway.TCPRoutes.InitWatch(routeRef2, nil)

	require.True(t, snap.APIGateway.TCPRoutes.Set(routeRef2, &structs.TCPRouteConfigEntry{
		Kind: structs.TCPRoute,
		Name: "tcp-route-2",
		Services: []structs.TCPService{{
			Name: "backend-b",
			TLS:  &structs.GatewayServiceTLSConfig{SDS: &structs.GatewayTLSSDSConfig{ClusterName: "sds-cluster", CertResource: "service-b-cert"}},
		}},
	}))

	listenerCfg := structs.APIGatewayListener{
		Name:     "tcp-listener",
		Protocol: structs.ListenerProtocolTCP,
	}
	resolvedTLSCfg, err := resolveAPIListenerTLSConfig(snap.APIGateway.TLSConfig, listenerCfg.TLS)
	require.NoError(t, err)

	_, err = collectAPIGatewayServiceSDSOverridesWithResolvedTLS(snap, readyListener{
		listenerCfg: listenerCfg,
		routeReferences: map[structs.ResourceReference]struct{}{
			routeRef1: {},
			routeRef2: {},
		},
	}, resolvedTLSCfg)

	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple TCP route TLS.SDS overrides")
}

func TestCollectAPIGatewayServiceSDSOverrides_TCPRouteInheritsGatewaySDSCluster(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
		entry.Listeners = []structs.APIGatewayListener{{
			Name:     "tcp-listener",
			Protocol: structs.ListenerProtocolTCP,
			Port:     9000,
		}}
		bound.Listeners = []structs.BoundAPIGatewayListener{{
			Name: "tcp-listener",
			Routes: []structs.ResourceReference{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
			}},
		}}
	}, []structs.BoundRoute{
		&structs.TCPRouteConfigEntry{
			Kind: structs.TCPRoute,
			Name: "tcp-route",
			Parents: []structs.ResourceReference{{
				Kind: structs.APIGateway,
				Name: "api-gateway",
			}},
			Services: []structs.TCPService{{
				Name: "backend",
				TLS:  &structs.GatewayServiceTLSConfig{SDS: &structs.GatewayTLSSDSConfig{CertResource: "service-cert"}},
			}},
		},
	}, nil, nil)
	if snap == nil {
		t.Fatal("expected non-nil config snapshot")
	}

	snap.APIGateway.TLSConfig = structs.GatewayTLSConfig{
		SDS: &structs.GatewayTLSSDSConfig{ClusterName: "gateway-sds-cluster"},
	}

	listenerCfg := structs.APIGatewayListener{
		Name:     "tcp-listener",
		Protocol: structs.ListenerProtocolTCP,
	}
	resolvedTLSCfg, err := resolveAPIListenerTLSConfig(snap.APIGateway.TLSConfig, listenerCfg.TLS)
	require.NoError(t, err)

	overrides, err := collectAPIGatewayServiceSDSOverridesWithResolvedTLS(snap, readyListener{
		listenerCfg: listenerCfg,
		routeReferences: map[structs.ResourceReference]struct{}{
			{Kind: structs.TCPRoute, Name: "tcp-route"}: {},
		},
	}, resolvedTLSCfg)

	require.NoError(t, err)
	require.Len(t, overrides, 1)
	require.Equal(t, "gateway-sds-cluster", overrides[0].SDS.ClusterName)
	require.Equal(t, "service-cert", overrides[0].SDS.CertResource)
}

func TestResolveAPIListenerTLSConfig_GatewayAndListenerMerge(t *testing.T) {
	cfg, err := resolveAPIListenerTLSConfig(
		structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "gateway-cluster",
				CertResource: "gateway-default-cert",
			},
			TLSMinVersion: types.TLSv1_2,
		},
		structs.APIGatewayTLSConfiguration{
			SDS:        &structs.GatewayTLSSDSConfig{CertResource: "listener-cert"},
			MaxVersion: types.TLSv1_3,
		},
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.SDS)
	require.Equal(t, "gateway-cluster", cfg.SDS.ClusterName)
	require.Equal(t, "listener-cert", cfg.SDS.CertResource)
	require.Equal(t, types.TLSv1_2, cfg.TLSMinVersion)
	require.Equal(t, types.TLSv1_3, cfg.TLSMaxVersion)
}

func TestResolveAPIListenerTLSConfig_InvalidMergedSDS(t *testing.T) {
	t.Run("gateway cluster only default is allowed", func(t *testing.T) {
		cfg, err := resolveAPIListenerTLSConfig(
			structs.GatewayTLSConfig{
				SDS: &structs.GatewayTLSSDSConfig{ClusterName: "gateway-sds-cluster"},
			},
			structs.APIGatewayTLSConfiguration{},
		)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.SDS)
		require.Equal(t, "gateway-sds-cluster", cfg.SDS.ClusterName)
		require.Empty(t, cfg.SDS.CertResource)
	})

	t.Run("cert resource without cluster name", func(t *testing.T) {
		cfg, err := resolveAPIListenerTLSConfig(
			structs.GatewayTLSConfig{},
			structs.APIGatewayTLSConfiguration{
				SDS: &structs.GatewayTLSSDSConfig{CertResource: "listener-cert"},
			},
		)

		require.Error(t, err)
		require.Nil(t, cfg)
		require.Contains(t, err.Error(), "ClusterName is required")
	})

	t.Run("cluster name without cert resource", func(t *testing.T) {
		cfg, err := resolveAPIListenerTLSConfig(
			structs.GatewayTLSConfig{},
			structs.APIGatewayTLSConfiguration{
				SDS: &structs.GatewayTLSSDSConfig{ClusterName: "sds-cluster"},
			},
		)

		require.Error(t, err)
		require.Nil(t, cfg)
		require.Contains(t, err.Error(), "CertResource is required")
	})
}

// Made with Bob
