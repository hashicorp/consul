// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

// countingLeafSource is a minimal LeafCertificate data source that records how
// many times Notify is called and the DNS SANs of the most recent request.
type countingLeafSource struct {
	count   int
	lastReq *leafcert.ConnectCALeafRequest
}

func (c *countingLeafSource) Notify(_ context.Context, req *leafcert.ConnectCALeafRequest, _ string, _ chan<- UpdateEvent) error {
	c.count++
	c.lastReq = req
	return nil
}

func testAPIGatewayHandler(t *testing.T, leaf LeafCertificate) *handlerAPIGateway {
	t.Helper()
	return &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				source:    &structs.QuerySource{Datacenter: "dc1"},
				dnsConfig: DNSConfig{Domain: "consul"},
				dataSources: DataSources{
					LeafCertificate: leaf,
				},
			},
			ch: make(chan UpdateEvent, 1),
		},
	}
}

// TestGenerateAPIGatewayDNSSANs_NoTLS verifies that generateAPIGatewayDNSSANs
// returns nil when the global TLS flag is not set. The leaf cert is still
// issued for outbound mTLS, but with no DNS SANs — which prevents unnecessary
// XFCC header fields that would otherwise break RBAC intentions on the
// destination service.
func TestGenerateAPIGatewayDNSSANs_NoTLS(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// entry.TLS.Enabled is false by default — no global TLS flag set.
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "http-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8080,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "http-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8080,
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.Nil(t, sans, "no DNS SANs should be generated when APIGatewayConfigEntry.TLS.Enabled is false")
}

// TestGenerateAPIGatewayDNSSANs_GlobalTLSEnabled verifies that DNS SANs are
// injected when APIGatewayConfigEntry.TLS.Enabled is true. Listener-level
// certificate references are irrelevant to this decision — they only control
// which custom cert is presented, not whether TLS is active.
func TestGenerateAPIGatewayDNSSANs_GlobalTLSEnabled(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			entry.TLS = structs.GatewayTLSConfig{Enabled: true}
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when APIGatewayConfigEntry.TLS.Enabled is true")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_GatewayLevelMinVersion verifies that DNS SANs are
// injected when gateway-level TLS MinVersion is configured, even without the
// global Enabled flag. This tests gateway-level TLS parameter path.
func TestGenerateAPIGatewayDNSSANs_GatewayLevelMinVersion(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global Enabled flag, but gateway has TLS MinVersion configured
			entry.TLS = structs.GatewayTLSConfig{TLSMinVersion: "TLSv1_2"}
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when gateway has TLS MinVersion")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_GatewayLevelMaxVersion verifies that DNS SANs are
// injected when gateway-level TLS MaxVersion is configured.
func TestGenerateAPIGatewayDNSSANs_GatewayLevelMaxVersion(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global Enabled flag, but gateway has TLS MaxVersion configured
			entry.TLS = structs.GatewayTLSConfig{TLSMaxVersion: "TLSv1_3"}
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when gateway has TLS MaxVersion")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_GatewayLevelCipherSuites verifies that DNS SANs are
// injected when gateway-level TLS CipherSuites is configured.
func TestGenerateAPIGatewayDNSSANs_GatewayLevelCipherSuites(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global Enabled flag, but gateway has CipherSuites configured
			entry.TLS = structs.GatewayTLSConfig{
				CipherSuites: []types.TLSCipherSuite{
					types.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				},
			}
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when gateway has TLS CipherSuites")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_GatewayLevelSDS verifies that DNS SANs are
// injected when gateway-level TLS SDS is configured with a CertResource.
func TestGenerateAPIGatewayDNSSANs_GatewayLevelSDS(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global Enabled flag, but gateway has SDS configured with CertResource
			entry.TLS = structs.GatewayTLSConfig{
				SDS: &structs.GatewayTLSSDSConfig{
					ClusterName:  "vault-sds",
					CertResource: "secret/consul/tls",
				},
			}
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when gateway has SDS with CertResource")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// hostnames, and that the result is sorted for deterministic cert requests.
func TestGenerateAPIGatewayDNSSANs(t *testing.T) {
	route := &structs.HTTPRouteConfigEntry{
		Kind:      structs.HTTPRoute,
		Name:      "r1",
		Hostnames: []string{"web.example.com"},
		Parents:   []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
		Rules:     []structs.HTTPRouteRule{{Services: []structs.HTTPService{{Name: "web"}}}},
	}
	ref := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "r1"}

	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			entry.TLS = structs.GatewayTLSConfig{Enabled: true}
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
				Hostname: "listener.example.com",
			}}
			// BoundAPIGatewayListener carries a copy of the api-gateway listener
			// config fields (populated by the controller at reconcile time), so
			// the test must mirror that copy for generateAPIGatewayDNSSANs to see
			// the listener hostname.
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
				Hostname: "listener.example.com",
				Routes:   []structs.ResourceReference{ref},
			}}
		}, []structs.BoundRoute{route}, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
	require.Contains(t, sans, "web.example.com", "route hostnames must appear as leaf SANs")
	require.Contains(t, sans, "listener.example.com", "listener hostnames must appear as leaf SANs")
	require.True(t, sort.StringsAreSorted(sans), "SANs must be sorted for deterministic cert requests")
}

// TestGenerateAPIGatewayDNSSANs_TrimsTrailingDot verifies that an FQDN-form DNS
// domain (stored with a trailing dot, e.g. "consul.") yields valid wildcard SANs
// without a trailing dot. A trailing dot is rejected by strict TLS verifiers
// (e.g. macOS Secure Transport: "unsupported or invalid name syntax").
func TestGenerateAPIGatewayDNSSANs_TrimsTrailingDot(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			entry.TLS = structs.GatewayTLSConfig{Enabled: true}
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
		}, nil, nil, nil)

	h := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				source:    &structs.QuerySource{Datacenter: "dc1"},
				dnsConfig: DNSConfig{Domain: "consul.", AltDomain: "alt.consul."},
			},
		},
	}

	sans := h.generateAPIGatewayDNSSANs(snap)

	for _, san := range sans {
		require.False(t, strings.HasSuffix(san, "."),
			"SAN %q must not end with a trailing dot (invalid for strict TLS verifiers)", san)
	}
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
	require.Contains(t, sans, "*.api-gateway.alt.consul")
}

// TestWatchIngressLeafCert_RewatchOnSANChange verifies the cert-watch guard:
// calling watchIngressLeafCert with an unchanged SAN set is a no-op, while a
// changed SAN set (e.g. a newly-seen route hostname) re-establishes the watch.
// This covers the previously-missing re-watch on route updates.
func TestWatchIngressLeafCert_RewatchOnSANChange(t *testing.T) {
	leaf := &countingLeafSource{}
	h := testAPIGatewayHandler(t, leaf)
	h.service = "api-gateway"

	snap := newTestAPIGatewaySnapshot()
	// Enable global TLS so apiGatewayTLSServingEnabled returns true and SANs
	// are generated. Without this the guard returns nil on every call, which
	// means every call looks like a SAN change and the no-rewatch assertion
	// would never be exercised correctly.
	snap.APIGateway.TLSConfig.Enabled = true

	// First watch: establishes with the base (wildcard-only) SANs.
	require.NoError(t, h.watchIngressLeafCert(context.Background(), snap))
	require.Equal(t, 1, leaf.count)
	require.NotNil(t, leaf.lastReq)
	require.Contains(t, leaf.lastReq.DNSSAN, "*.api-gateway.consul")
	require.NotContains(t, leaf.lastReq.DNSSAN, "web.example.com")

	// Second watch with identical SANs: must be a no-op (no re-issue).
	require.NoError(t, h.watchIngressLeafCert(context.Background(), snap))
	require.Equal(t, 1, leaf.count, "identical SANs must not re-establish the leaf watch")

	// A route now contributes a new hostname SAN: the watch must re-fire.
	ref := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "r1"}
	snap.APIGateway.HTTPRoutes.InitWatch(ref, nil)
	snap.APIGateway.HTTPRoutes.Set(ref, &structs.HTTPRouteConfigEntry{
		Kind:      structs.HTTPRoute,
		Name:      "r1",
		Hostnames: []string{"web.example.com"},
	})

	require.NoError(t, h.watchIngressLeafCert(context.Background(), snap))
	require.Equal(t, 2, leaf.count, "a new route hostname SAN must re-establish the leaf watch")
	require.Contains(t, leaf.lastReq.DNSSAN, "web.example.com")
}

// TestWatchIngressLeafCert_NoTLSNoSANs verifies that when no listener terminates
// TLS the leaf cert watch is still established (for outbound mTLS) but with a
// nil/empty DNSSAN slice — no wildcard SANs are included.
func TestWatchIngressLeafCert_NoTLSNoSANs(t *testing.T) {
	leaf := &countingLeafSource{}
	h := testAPIGatewayHandler(t, leaf)
	h.service = "api-gateway"

	snap := newTestAPIGatewaySnapshot()
	// No TLS listeners — BoundListeners is empty from newTestAPIGatewaySnapshot.

	require.NoError(t, h.watchIngressLeafCert(context.Background(), snap))
	require.Equal(t, 1, leaf.count, "leaf cert watch must still be established without TLS")
	require.Empty(t, leaf.lastReq.DNSSAN, "no DNS SANs should be requested when no listener terminates TLS")
}

// TestGenerateAPIGatewayDNSSANs_ListenerLevelCertificates verifies that DNS SANs are
// injected when a listener has custom certificates configured at the listener level,
// even without a global TLS.Enabled flag. This tests the listener-level certificate path.
func TestGenerateAPIGatewayDNSSANs_ListenerLevelCertificates(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global TLS flag — entry.TLS.Enabled is false by default
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			// BUT the listener has custom certificates at the listener level
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
				TLS: structs.APIGatewayTLSConfiguration{
					Certificates: []structs.ResourceReference{{
						Kind: structs.InlineCertificate,
						Name: "custom-cert",
					}},
				},
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when listener has custom certificates")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_ListenerLevelSDS verifies that DNS SANs are
// injected when a listener has an SDS source configured at the listener level,
// even without a global TLS.Enabled flag. This tests the listener-level SDS path.
func TestGenerateAPIGatewayDNSSANs_ListenerLevelSDS(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global TLS flag — entry.TLS.Enabled is false by default
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			// BUT the listener has SDS source with CertResource at the listener level
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
				TLS: structs.APIGatewayTLSConfiguration{
					SDS: &structs.GatewayTLSSDSConfig{
						ClusterName:  "vault-sds",
						CertResource: "secret/consul/listener-tls",
					},
				},
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when listener has SDS with CertResource")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_ListenerLevelMinVersion verifies that DNS SANs are
// injected when a listener has TLS MinVersion configured at the listener level,
// even without custom certificates or SDS. This tests listener-level TLS parameter path.
func TestGenerateAPIGatewayDNSSANs_ListenerLevelMinVersion(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global TLS flag — entry.TLS.Enabled is false by default
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			// BUT the listener has TLS MinVersion configured at listener level
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
				TLS: structs.APIGatewayTLSConfiguration{
					MinVersion: "TLSv1_2",
				},
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when listener has TLS MinVersion")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_ListenerLevelMaxVersion verifies that DNS SANs are
// injected when a listener has TLS MaxVersion configured at the listener level,
// even without custom certificates or SDS.
func TestGenerateAPIGatewayDNSSANs_ListenerLevelMaxVersion(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global TLS flag — entry.TLS.Enabled is false by default
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			// BUT the listener has TLS MaxVersion configured at listener level
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
				TLS: structs.APIGatewayTLSConfiguration{
					MaxVersion: "TLSv1_3",
				},
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when listener has TLS MaxVersion")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// TestGenerateAPIGatewayDNSSANs_ListenerLevelCipherSuites verifies that DNS SANs are
// injected when a listener has TLS CipherSuites configured at the listener level,
// even without custom certificates or SDS.
func TestGenerateAPIGatewayDNSSANs_ListenerLevelCipherSuites(t *testing.T) {
	snap := TestConfigSnapshotAPIGateway(t, "default", nil,
		func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
			// No global TLS flag — entry.TLS.Enabled is false by default
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
			}}
			// BUT the listener has TLS CipherSuites configured at listener level
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:     "https-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8443,
				TLS: structs.APIGatewayTLSConfiguration{
					CipherSuites: []types.TLSCipherSuite{
						types.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					},
				},
			}}
		}, nil, nil, nil)

	h := testAPIGatewayHandler(t, nil)
	sans := h.generateAPIGatewayDNSSANs(snap)

	require.NotNil(t, sans, "DNS SANs must be generated when listener has TLS CipherSuites")
	require.Contains(t, sans, "*.api-gateway.consul")
	require.Contains(t, sans, "*.api-gateway.dc1.consul")
}

// newTestAPIGatewaySnapshot builds a minimal API gateway snapshot with the
// gateway config loaded and empty route/upstream maps, sufficient to exercise
// watchIngressLeafCert / generateAPIGatewayDNSSANs.
func newTestAPIGatewaySnapshot() *ConfigSnapshot {
	snap := &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway}
	snap.APIGateway.GatewayConfigLoaded = true
	snap.APIGateway.BoundListeners = map[string]structs.BoundAPIGatewayListener{}
	snap.APIGateway.Upstreams = make(listenerRouteUpstreams)
	snap.APIGateway.HTTPRoutes = watch.NewMap[structs.ResourceReference, *structs.HTTPRouteConfigEntry]()
	return snap
}

// noopConfigEntrySource is a ConfigEntry data source whose Notify is a no-op.
// It lets us drive handleGatewayConfigUpdate without a live backend.
type noopConfigEntrySource struct{}

func (noopConfigEntrySource) Notify(_ context.Context, _ *structs.ConfigEntryQuery, _ string, _ chan<- UpdateEvent) error {
	return nil
}

// TestInlineCertSurvivesGatewayReconcile is a regression test for the bug where
// attached inline certificates were dropped from the snapshot whenever the
// gateway reconciled. handleGatewayConfigUpdate used InitWatch (which wipes the
// stored value) for certificates while routes used UpdateWatch (which preserves
// it). With multiple certificates this collapsed all custom SNI filter chains
// down to the Connect leaf catch-all. This is the certificate analogue of the
// route fix in #23562.
func TestInlineCertSurvivesGatewayReconcile(t *testing.T) {
	h := testAPIGatewayHandler(t, &countingLeafSource{})
	h.service = "api-gateway"
	h.dataSources.ConfigEntry = noopConfigEntrySource{}

	snap := newTestAPIGatewaySnapshot()
	snap.APIGateway.BoundListeners = map[string]structs.BoundAPIGatewayListener{}
	snap.APIGateway.TCPRoutes = watch.NewMap[structs.ResourceReference, *structs.TCPRouteConfigEntry]()
	snap.APIGateway.InlineCertificates = watch.NewMap[structs.ResourceReference, *structs.InlineCertificateConfigEntry]()
	snap.APIGateway.FileSystemCertificates = watch.NewMap[structs.ResourceReference, *structs.FileSystemCertificateConfigEntry]()
	snap.APIGateway.UpstreamsSet = make(routeUpstreamSet)

	certRefs := []structs.ResourceReference{
		{Kind: structs.InlineCertificate, Name: "web"},
		{Kind: structs.InlineCertificate, Name: "api"},
	}
	boundEntry := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "api-gateway",
		Listeners: []structs.BoundAPIGatewayListener{{
			Name:         "http-listener",
			Certificates: certRefs,
		}},
	}
	boundEvent := UpdateEvent{
		CorrelationID: boundGatewayConfigWatchID,
		Result:        &structs.ConfigEntryResponse{Entry: boundEntry},
	}

	ctx := context.Background()

	// 1) Initial bound-gateway update wires up the cert watches.
	require.NoError(t, h.handleGatewayConfigUpdate(ctx, boundEvent, snap, boundGatewayConfigWatchID))

	// 2) Both certificate values arrive and are stored in the snapshot.
	for _, ref := range certRefs {
		certEvent := UpdateEvent{
			CorrelationID: inlineCertificateConfigWatchID,
			Result: &structs.ConfigEntryResponse{Entry: &structs.InlineCertificateConfigEntry{
				Kind: structs.InlineCertificate,
				Name: ref.Name,
			}},
		}
		require.NoError(t, h.handleInlineCertConfigUpdate(ctx, certEvent, snap))
	}
	for _, ref := range certRefs {
		_, ok := snap.APIGateway.InlineCertificates.Get(ref)
		require.True(t, ok, "cert %q must be stored after its update", ref.Name)
	}

	// 3) A subsequent gateway reconcile (e.g. status/route churn) MUST NOT drop
	// the previously-stored certificate values.
	require.NoError(t, h.handleGatewayConfigUpdate(ctx, boundEvent, snap, boundGatewayConfigWatchID))

	for _, ref := range certRefs {
		_, ok := snap.APIGateway.InlineCertificates.Get(ref)
		require.True(t, ok, "cert %q must survive a gateway reconcile (UpdateWatch, not InitWatch)", ref.Name)
	}
}
