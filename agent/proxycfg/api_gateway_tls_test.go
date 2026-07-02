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

// TestGenerateAPIGatewayDNSSANs verifies the leaf-cert DNS SANs include the
// "*.api-gateway.<domain>" wildcards plus explicit listener and route
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
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "http-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8080,
				Hostname: "listener.example.com",
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{
				Name:   "http-listener",
				Routes: []structs.ResourceReference{ref},
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
			entry.Listeners = []structs.APIGatewayListener{{
				Name:     "http-listener",
				Protocol: structs.ListenerProtocolHTTP,
				Port:     8080,
			}}
			bound.Listeners = []structs.BoundAPIGatewayListener{{Name: "http-listener"}}
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

// newTestAPIGatewaySnapshot builds a minimal API gateway snapshot with the
// gateway config loaded and empty route/upstream maps, sufficient to exercise
// watchIngressLeafCert / generateAPIGatewayDNSSANs.
func newTestAPIGatewaySnapshot() *ConfigSnapshot {
	snap := &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway}
	snap.APIGateway.GatewayConfigLoaded = true
	snap.APIGateway.Listeners = map[string]structs.APIGatewayListener{}
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
