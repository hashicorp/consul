// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// notifyCall records one call to a data source's Notify method.
type notifyCall struct {
	ctx           context.Context
	correlationID string
}

// countingWatchRecorder records every Notify call in order, so that repeat
// registrations of the same correlation ID can be counted.
type countingWatchRecorder struct {
	mu    sync.Mutex
	calls []notifyCall
}

func (r *countingWatchRecorder) record(ctx context.Context, correlationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, notifyCall{ctx: ctx, correlationID: correlationID})
}

func (r *countingWatchRecorder) callsFor(correlationID string) []notifyCall {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []notifyCall
	for _, c := range r.calls {
		if c.correlationID == correlationID {
			out = append(out, c)
		}
	}
	return out
}

type countingTypedRecorder[ReqType any] struct {
	recorder *countingWatchRecorder
}

func (r countingTypedRecorder[ReqType]) Notify(ctx context.Context, _ ReqType, correlationID string, _ chan<- UpdateEvent) error {
	r.recorder.record(ctx, correlationID)
	return nil
}

func countingWatches(sc *stateConfig) *countingWatchRecorder {
	rec := &countingWatchRecorder{}

	sc.dataSources = DataSources{
		CARoots:                         countingTypedRecorder[*structs.DCSpecificRequest]{rec},
		CompiledDiscoveryChain:          countingTypedRecorder[*structs.DiscoveryChainRequest]{rec},
		ConfigEntry:                     countingTypedRecorder[*structs.ConfigEntryQuery]{rec},
		ConfigEntryList:                 countingTypedRecorder[*structs.ConfigEntryQuery]{rec},
		Datacenters:                     countingTypedRecorder[*structs.DatacentersRequest]{rec},
		FederationStateListMeshGateways: countingTypedRecorder[*structs.DCSpecificRequest]{rec},
		GatewayServices:                 countingTypedRecorder[*structs.ServiceSpecificRequest]{rec},
		ServiceGateways:                 countingTypedRecorder[*structs.ServiceSpecificRequest]{rec},
		Health:                          countingTypedRecorder[*structs.ServiceSpecificRequest]{rec},
		HTTPChecks:                      countingTypedRecorder[*cachetype.ServiceHTTPChecksRequest]{rec},
		Intentions:                      countingTypedRecorder[*structs.ServiceSpecificRequest]{rec},
		IntentionUpstreams:              countingTypedRecorder[*structs.ServiceSpecificRequest]{rec},
		IntentionUpstreamsDestination:   countingTypedRecorder[*structs.ServiceSpecificRequest]{rec},
		InternalServiceDump:             countingTypedRecorder[*structs.ServiceDumpRequest]{rec},
		LeafCertificate:                 countingTypedRecorder[*leafcert.ConnectCALeafRequest]{rec},
		PeeringList:                     countingTypedRecorder[*cachetype.PeeringListRequest]{rec},
		PeeredUpstreams:                 countingTypedRecorder[*structs.PartitionSpecificRequest]{rec},
		PreparedQuery:                   countingTypedRecorder[*structs.PreparedQueryExecuteRequest]{rec},
		ResolvedServiceConfig:           countingTypedRecorder[*structs.ServiceConfigRequest]{rec},
		ServiceList:                     countingTypedRecorder[*structs.DCSpecificRequest]{rec},
		TrustBundle:                     countingTypedRecorder[*cachetype.TrustBundleReadRequest]{rec},
		TrustBundleList:                 countingTypedRecorder[*cachetype.TrustBundleListRequest]{rec},
		ExportedPeeredServices:          countingTypedRecorder[*structs.DCSpecificRequest]{rec},
	}

	return rec
}

// TestPeeredUpstreams_RepeatRegistrationLeaksWatches asserts that repeated
// peered-upstreams updates - which happen every time the Internal.PeeredUpstreams
// blocking query returns a new index - do not register the same health watch
// over and over.
//
// Each registration starts a goroutine in submatview.Store.NotifyCallback (or
// cache.notifyBlockingQuery) that only ever exits when its context is
// cancelled. setupWatchesForPeeredUpstream passes the long-lived state context
// and records a nil cancel func, so nothing can ever reclaim them: one
// goroutine per peered upstream, per update, for the lifetime of the agent.
//
// See https://github.com/hashicorp/consul/issues/23931.
func TestPeeredUpstreams_RepeatRegistrationLeaksWatches(t *testing.T) {
	ns := structs.TestNodeServiceProxy(t)
	ns.Proxy.Mode = structs.ProxyModeTransparent
	ns.Proxy.Upstreams = nil

	proxyID := ProxyID{ServiceID: ns.CompoundServiceID()}

	sc := stateConfig{
		logger: testutil.Logger(t),
		source: &structs.QuerySource{Datacenter: "dc1"},
		dnsConfig: DNSConfig{
			Domain:    "consul.",
			AltDomain: "alt.consul.",
		},
	}
	rec := countingWatches(&sc)

	state, err := newState(proxyID, ns, testSource, aclToken, sc, rate.NewLimiter(rate.Inf, 0))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	state.cancel = cancel

	snap, err := state.handler.initialize(ctx)
	require.NoError(t, err)

	peered := &structs.IndexedPeeredServiceList{
		Services: []structs.PeeredServiceName{
			{Peer: "peer-a", ServiceName: structs.NewServiceName("payments", nil)},
			{Peer: "peer-a", ServiceName: structs.NewServiceName("refunds", nil)},
		},
	}

	uid := NewUpstreamIDFromPeeredServiceName(peered.Services[0])
	correlationID := upstreamPeerWatchIDPrefix + uid.String()

	// Three identical updates, as the blocking query would deliver after each
	// index bump on the exporting side.
	const updates = 3
	for i := 0; i < updates; i++ {
		require.NoError(t, state.handler.handleUpdate(ctx, UpdateEvent{
			CorrelationID: peeredUpstreamsID,
			Result:        peered,
		}, &snap))
	}

	calls := rec.callsFor(correlationID)
	require.Len(t, calls, 1,
		"health watch for peered upstream %q was registered %d times across %d identical updates; "+
			"every extra registration strands a watcher goroutine", uid, len(calls), updates)

	// Each registration must also be individually cancellable, otherwise
	// dropping the upstream cannot reclaim its goroutine.
	for i, c := range calls {
		require.NotNil(t, c.ctx, "call %d", i)
		require.NotNil(t, c.ctx.Done(), "call %d: watch context is not cancellable", i)
	}

	// And dropping the peered upstream entirely must cancel the watch.
	require.NoError(t, state.handler.handleUpdate(ctx, UpdateEvent{
		CorrelationID: peeredUpstreamsID,
		Result:        &structs.IndexedPeeredServiceList{},
	}, &snap))

	select {
	case <-calls[0].ctx.Done():
	default:
		t.Fatalf("health watch for peered upstream %q was not cancelled after the upstream disappeared", uid)
	}
}
