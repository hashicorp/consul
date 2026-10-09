// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// peeredWatchCall is one Notify attempt, including attempts that the test
// forced to fail, so that tests can assert both registration counts and the
// lifecycle of each registration's context.
type peeredWatchCall struct {
	correlationID string
	ctx           context.Context
	request       any
	err           error
}

// peeredWatchRecorder records every Notify call. The existing watchRecorder
// keeps only the latest request per correlation ID and ignores contexts, which
// hides duplicate registrations of the same correlation ID.
type peeredWatchRecorder struct {
	mu       sync.Mutex
	calls    []peeredWatchCall
	failures map[string]error
}

func newPeeredWatchRecorder() *peeredWatchRecorder {
	return &peeredWatchRecorder{failures: make(map[string]error)}
}

func (r *peeredWatchRecorder) notify(ctx context.Context, req any, correlationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	err := r.failures[correlationID]
	delete(r.failures, correlationID)
	r.calls = append(r.calls, peeredWatchCall{
		correlationID: correlationID,
		ctx:           ctx,
		request:       req,
		err:           err,
	})
	return err
}

// failNext makes the next Notify call for correlationID fail with err.
func (r *peeredWatchRecorder) failNext(correlationID string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures[correlationID] = err
}

// attempts returns every Notify call for correlationID, including failures.
func (r *peeredWatchRecorder) attempts(correlationID string) []peeredWatchCall {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []peeredWatchCall
	for _, call := range r.calls {
		if call.correlationID == correlationID {
			out = append(out, call)
		}
	}
	return out
}

// registrations returns the successful Notify calls for correlationID.
func (r *peeredWatchRecorder) registrations(correlationID string) []peeredWatchCall {
	var out []peeredWatchCall
	for _, call := range r.attempts(correlationID) {
		if call.err == nil {
			out = append(out, call)
		}
	}
	return out
}

// active counts successful registrations for correlationID whose contexts
// have not been cancelled. These correspond to live watch goroutines.
func (r *peeredWatchRecorder) active(correlationID string) int {
	count := 0
	for _, call := range r.registrations(correlationID) {
		if call.ctx.Err() == nil {
			count++
		}
	}
	return count
}

// activeWithPrefix counts live registrations whose correlation ID has prefix.
func (r *peeredWatchRecorder) activeWithPrefix(prefix string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, call := range r.calls {
		if call.err == nil && strings.HasPrefix(call.correlationID, prefix) && call.ctx.Err() == nil {
			count++
		}
	}
	return count
}

type peeredRecordingSource[ReqType any] struct {
	recorder *peeredWatchRecorder
}

func (s peeredRecordingSource[ReqType]) Notify(ctx context.Context, req ReqType, correlationID string, _ chan<- UpdateEvent) error {
	return s.recorder.notify(ctx, req, correlationID)
}

// peeredUpstreamHarness drives a transparent connect-proxy handler directly,
// mirroring how state.run serializes updates through handleUpdate.
type peeredUpstreamHarness struct {
	t      *testing.T
	ctx    context.Context
	cancel context.CancelFunc
	state  *state
	snap   ConfigSnapshot
	index  uint64
}

func newPeeredUpstreamHarness(t *testing.T, recorder *peeredWatchRecorder, proxyName string, upstreams structs.Upstreams) *peeredUpstreamHarness {
	t.Helper()

	ns := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      proxyName + "-sidecar-proxy",
		Service: proxyName + "-sidecar-proxy",
		Address: "10.0.1.1",
		Port:    21000,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: proxyName,
			Mode:                   structs.ProxyModeTransparent,
			Upstreams:              upstreams,
		},
	}

	sc := stateConfig{
		logger: testutil.Logger(t),
		source: &structs.QuerySource{Datacenter: "dc1"},
		dnsConfig: DNSConfig{
			Domain:    "consul.",
			AltDomain: "alt.consul.",
		},
	}
	recordWatches(&sc)
	sc.dataSources.Health = peeredRecordingSource[*structs.ServiceSpecificRequest]{recorder}
	sc.dataSources.TrustBundle = peeredRecordingSource[*cachetype.TrustBundleReadRequest]{recorder}
	sc.dataSources.InternalServiceDump = peeredRecordingSource[*structs.ServiceDumpRequest]{recorder}

	proxyID := ProxyID{ServiceID: ns.CompoundServiceID()}
	s, err := newState(proxyID, &ns, testSource, aclToken, sc, rate.NewLimiter(rate.Inf, 0))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s.cancel = cancel

	snap, err := s.handler.initialize(ctx)
	require.NoError(t, err)

	return &peeredUpstreamHarness{t: t, ctx: ctx, cancel: cancel, state: s, snap: snap}
}

func (h *peeredUpstreamHarness) handle(event UpdateEvent) error {
	return h.state.handler.handleUpdate(h.ctx, event, &h.snap)
}

func (h *peeredUpstreamHarness) mustHandle(event UpdateEvent) {
	h.t.Helper()
	require.NoError(h.t, h.handle(event))
}

func (h *peeredUpstreamHarness) importedListEvent(services ...structs.PeeredServiceName) UpdateEvent {
	h.index++
	return UpdateEvent{
		CorrelationID: peeredUpstreamsID,
		Result: &structs.IndexedPeeredServiceList{
			Services:  services,
			QueryMeta: structs.QueryMeta{Index: h.index},
		},
	}
}

func (h *peeredUpstreamHarness) deliverImported(services ...structs.PeeredServiceName) error {
	return h.handle(h.importedListEvent(services...))
}

func (h *peeredUpstreamHarness) mustDeliverImported(services ...structs.PeeredServiceName) {
	h.t.Helper()
	require.NoError(h.t, h.deliverImported(services...))
}

func (h *peeredUpstreamHarness) peerEndpoints() map[UpstreamID]structs.CheckServiceNodes {
	out := make(map[UpstreamID]structs.CheckServiceNodes)
	h.snap.ConnectProxy.PeerUpstreamEndpoints.ForEachKey(func(uid UpstreamID) bool {
		nodes, _ := h.snap.ConnectProxy.PeerUpstreamEndpoints.Get(uid)
		out[uid] = nodes
		return true
	})
	return out
}

func importedService(name, peer string) structs.PeeredServiceName {
	return structs.PeeredServiceName{
		ServiceName: structs.NewServiceName(name, nil),
		Peer:        peer,
	}
}

func importedUID(name, peer string) UpstreamID {
	return NewUpstreamIDFromPeeredServiceName(importedService(name, peer))
}

func peerHealthWatchID(uid UpstreamID) string {
	return upstreamPeerWatchIDPrefix + uid.String()
}

func peerHealthEvent(uid UpstreamID, address string) UpdateEvent {
	return UpdateEvent{
		CorrelationID: peerHealthWatchID(uid),
		Result: &structs.IndexedCheckServiceNodes{
			Nodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:     "node-" + uid.Name,
						Address:  address,
						PeerName: uid.Peer,
					},
					Service: &structs.NodeService{
						ID:       uid.Name,
						Service:  uid.Name,
						Address:  address,
						PeerName: uid.Peer,
					},
				},
			},
		},
	}
}

func requireCancelled(t *testing.T, calls []peeredWatchCall) {
	t.Helper()
	for i, call := range calls {
		require.Error(t, call.ctx.Err(), "watch %d for %q should be cancelled", i, call.correlationID)
	}
}

const peerTrustBundleWatchPrefix = peerTrustBundleIDPrefix

func TestConnectProxyPeeredUpstreamWatchLifecycle(t *testing.T) {
	db := importedService("db", "peer-a")
	dbUID := importedUID("db", "peer-a")
	api := importedService("api", "peer-a")
	apiUID := importedUID("api", "peer-a")
	tmp := importedService("tmp", "peer-a")
	tmpUID := importedUID("tmp", "peer-a")

	t.Run("initial registration creates exactly one health watch", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db)

		regs := rec.registrations(peerHealthWatchID(dbUID))
		require.Len(t, regs, 1)
		req, ok := regs[0].request.(*structs.ServiceSpecificRequest)
		require.True(t, ok)
		require.Equal(t, "db", req.ServiceName)
		require.Equal(t, "peer-a", req.PeerName)
		require.Equal(t, "dc1", req.Datacenter)
		require.True(t, req.Connect)
		require.Equal(t, aclToken, req.Token)
		require.True(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(dbUID))
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)))
	})

	t.Run("repeated imported list does not register again", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		for i := 0; i < 5; i++ {
			h.mustDeliverImported(db)
		}

		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1)
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)))
	})

	t.Run("duplicate and reordered entries register each upstream once", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db, api, db)
		h.mustDeliverImported(api, db)
		h.mustDeliverImported(db, api)

		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1)
		require.Len(t, rec.registrations(peerHealthWatchID(apiUID)), 1)
		require.Equal(t, 2, rec.activeWithPrefix(upstreamPeerWatchIDPrefix))
	})

	t.Run("unrelated churn keeps one stable watch and cancels removed watches", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db)
		const cycles = 50
		for i := 0; i < cycles; i++ {
			h.mustDeliverImported(db, tmp)
			h.mustDeliverImported(db)

			require.Equal(t, 1, rec.activeWithPrefix(upstreamPeerWatchIDPrefix),
				"cycle %d: live peered health watches must match the current upstream set", i+1)
		}

		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1,
			"the unchanged upstream must not be re-registered on unrelated list changes")
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)))

		tmpRegs := rec.registrations(peerHealthWatchID(tmpUID))
		require.Len(t, tmpRegs, cycles)
		requireCancelled(t, tmpRegs)
		require.False(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(tmpUID))
	})

	t.Run("endpoints survive repeated and unrelated list updates", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db)
		h.mustHandle(peerHealthEvent(dbUID, "10.0.0.1"))

		h.mustDeliverImported(db)
		h.mustDeliverImported(db, tmp)
		h.mustDeliverImported(db)

		nodes, ok := h.snap.ConnectProxy.PeerUpstreamEndpoints.Get(dbUID)
		require.True(t, ok, "existing endpoints must not be discarded by a list update")
		require.Len(t, nodes, 1)
		require.Equal(t, "10.0.0.1", nodes[0].Node.Address)

		h.mustHandle(peerHealthEvent(dbUID, "10.0.0.2"))
		nodes, ok = h.snap.ConnectProxy.PeerUpstreamEndpoints.Get(dbUID)
		require.True(t, ok)
		require.Equal(t, "10.0.0.2", nodes[0].Node.Address)
	})

	t.Run("removing an upstream cancels its watch and repeat removal is harmless", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db, api)
		h.mustDeliverImported(api)

		requireCancelled(t, rec.registrations(peerHealthWatchID(dbUID)))
		require.False(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(dbUID))
		require.Equal(t, 1, rec.active(peerHealthWatchID(apiUID)))

		h.mustDeliverImported(api)
		requireCancelled(t, rec.registrations(peerHealthWatchID(dbUID)))
		require.Len(t, rec.registrations(peerHealthWatchID(apiUID)), 1)
		require.Equal(t, 1, rec.active(peerHealthWatchID(apiUID)))
	})

	t.Run("empty list cancels dynamic watches without cancelling the owner", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db, api)
		h.mustDeliverImported()

		requireCancelled(t, rec.registrations(peerHealthWatchID(dbUID)))
		requireCancelled(t, rec.registrations(peerHealthWatchID(apiUID)))
		requireCancelled(t, rec.registrations(peerTrustBundleWatchPrefix+"peer-a"))
		require.Equal(t, 0, rec.activeWithPrefix(upstreamPeerWatchIDPrefix))
		require.Equal(t, 0, h.snap.ConnectProxy.PeerUpstreamEndpoints.Len())
		require.NoError(t, h.ctx.Err(), "the proxy owner's context must stay alive")
	})

	t.Run("re-adding a removed upstream creates one fresh watch", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db)
		h.mustDeliverImported()
		h.mustDeliverImported(db)
		h.mustDeliverImported(db)

		regs := rec.registrations(peerHealthWatchID(dbUID))
		require.Len(t, regs, 2)
		requireCancelled(t, regs[:1])
		require.NoError(t, regs[1].ctx.Err())
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)))
	})

	t.Run("late update for a removed upstream does not restore it", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db)
		h.mustDeliverImported()
		h.mustHandle(peerHealthEvent(dbUID, "10.0.0.9"))

		require.False(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(dbUID))
		_, ok := h.snap.ConnectProxy.PeerUpstreamEndpoints.Get(dbUID)
		require.False(t, ok)
	})

	t.Run("peers are isolated by identity", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)
		dbB := importedService("db", "peer-b")
		dbBUID := importedUID("db", "peer-b")

		h.mustDeliverImported(db, dbB)
		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1)
		require.Len(t, rec.registrations(peerHealthWatchID(dbBUID)), 1)

		h.mustDeliverImported(dbB)
		requireCancelled(t, rec.registrations(peerHealthWatchID(dbUID)))
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbBUID)))
		requireCancelled(t, rec.registrations(peerTrustBundleWatchPrefix+"peer-a"))
		require.Equal(t, 1, rec.active(peerTrustBundleWatchPrefix+"peer-b"))
	})

	t.Run("proxy owners are isolated from each other", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		first := newPeeredUpstreamHarness(t, rec, "web", nil)
		second := newPeeredUpstreamHarness(t, rec, "billing", nil)

		first.mustDeliverImported(db)
		second.mustDeliverImported(db)
		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 2)

		first.cancel()
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)),
			"closing one owner must not cancel another owner's watch")

		second.mustDeliverImported(db)
		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 2)
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)))
	})

	t.Run("owner teardown cancels every remaining watch", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(db)
		for i := 0; i < 10; i++ {
			h.mustDeliverImported(db, tmp)
			h.mustDeliverImported(db)
		}
		h.mustDeliverImported(db, api)

		h.cancel()
		require.Equal(t, 0, rec.activeWithPrefix(upstreamPeerWatchIDPrefix))
		require.Equal(t, 0, rec.activeWithPrefix(peerTrustBundleWatchPrefix))
	})

	t.Run("health registration failure cancels the attempt and can be retried", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)
		rec.failNext(peerHealthWatchID(dbUID), errors.New("injected health failure"))

		err := h.deliverImported(db)
		require.ErrorContains(t, err, "injected health failure")

		attempts := rec.attempts(peerHealthWatchID(dbUID))
		require.Len(t, attempts, 1)
		require.Error(t, attempts[0].ctx.Err(), "a failed registration must cancel its context")
		require.False(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(dbUID))
		require.NoError(t, h.ctx.Err())

		h.mustDeliverImported(db)
		regs := rec.registrations(peerHealthWatchID(dbUID))
		require.Len(t, regs, 1)
		require.NoError(t, regs[0].ctx.Err())
		require.True(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(dbUID))
	})

	t.Run("partial setup failure retries the missing trust bundle watch only", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)
		bundleID := peerTrustBundleWatchPrefix + "peer-a"
		rec.failNext(bundleID, errors.New("injected trust bundle failure"))

		err := h.deliverImported(db)
		require.ErrorContains(t, err, "injected trust bundle failure")
		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1)
		require.False(t, h.snap.ConnectProxy.UpstreamPeerTrustBundles.IsWatched("peer-a"))
		requireCancelled(t, rec.attempts(bundleID))

		h.mustDeliverImported(db)
		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1,
			"retry must reuse the existing health watch")
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)))
		require.Len(t, rec.registrations(bundleID), 1)
		require.Equal(t, 1, rec.active(bundleID))
		require.True(t, h.snap.ConnectProxy.UpstreamPeerTrustBundles.IsWatched("peer-a"))
	})

	t.Run("shared trust bundle lives until its last upstream is removed", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)
		bundleID := peerTrustBundleWatchPrefix + "peer-a"

		h.mustDeliverImported(db, api)
		h.mustDeliverImported(api, db)
		require.Len(t, rec.registrations(bundleID), 1)

		h.mustDeliverImported(api)
		require.Equal(t, 1, rec.active(bundleID))

		h.mustDeliverImported()
		requireCancelled(t, rec.registrations(bundleID))
		require.False(t, h.snap.ConnectProxy.UpstreamPeerTrustBundles.IsWatched("peer-a"))
	})

	t.Run("reusing a health watch still restores missing gateway setup", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)
		gatewayKey := GatewayKey{Datacenter: "dc1", Partition: h.state.serviceInstance.proxyID.PartitionOrDefault()}.String()
		gatewayID := "mesh-gateway:" + gatewayKey

		h.mustDeliverImported(db)
		h.mustDeliverImported(db)
		require.Len(t, rec.registrations(gatewayID), 1)
		require.True(t, h.snap.ConnectProxy.WatchedLocalGWEndpoints.IsWatched(gatewayKey))

		h.snap.ConnectProxy.WatchedLocalGWEndpoints.CancelWatch(gatewayKey)
		h.mustDeliverImported(db)

		require.True(t, h.snap.ConnectProxy.WatchedLocalGWEndpoints.IsWatched(gatewayKey))
		require.Len(t, rec.registrations(gatewayID), 2)
		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1)
	})

	t.Run("explicit upstream is not duplicated or removed by the imported list", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		explicit := structs.Upstreams{{DestinationName: "db", DestinationPeer: "peer-a"}}
		h := newPeeredUpstreamHarness(t, rec, "web", explicit)
		require.Equal(t, dbUID, NewUpstreamID(&h.state.serviceInstance.proxyCfg.Upstreams[0]))
		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1)

		h.mustHandle(peerHealthEvent(dbUID, "10.0.0.1"))
		h.mustDeliverImported(db)
		h.mustDeliverImported(db, api)

		require.Len(t, rec.registrations(peerHealthWatchID(dbUID)), 1)
		_, ok := h.snap.ConnectProxy.PeerUpstreamEndpoints.Get(dbUID)
		require.True(t, ok, "explicit upstream endpoints must survive imported list updates")

		h.mustDeliverImported()
		require.True(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(dbUID))
		require.Equal(t, 1, rec.active(peerHealthWatchID(dbUID)))
		requireCancelled(t, rec.registrations(peerHealthWatchID(apiUID)))
	})
}

// TestConnectProxyPeeredUpstreamWatchLifecycle_DiscoveryChainOverlap covers an
// imported service that is also a discovery-chain failover target. Both paths
// key peer endpoints by the same UpstreamID, but each owns its own watch.
func TestConnectProxyPeeredUpstreamWatchLifecycle_DiscoveryChainOverlap(t *testing.T) {
	billing := structs.NewServiceName("billing", nil)
	billingUID := NewUpstreamIDFromServiceName(billing)
	imported := importedService("billing", "cluster-01")
	peerUID := importedUID("billing", "cluster-01")
	healthID := peerHealthWatchID(peerUID)

	intentionUpstreams := func(services ...structs.ServiceName) UpdateEvent {
		return UpdateEvent{
			CorrelationID: intentionUpstreamsID,
			Result:        &structs.IndexedServiceList{Services: services},
		}
	}
	chainEvent := func(t *testing.T) UpdateEvent {
		return UpdateEvent{
			CorrelationID: "discovery-chain:" + billingUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: discoverychain.TestCompileConfigEntries(t, "billing", "default", "default", "dc1", "trustdomain.consul", nil,
					discoChainSetWithEntries(&structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "billing",
						Failover: map[string]structs.ServiceResolverFailover{
							"*": {Targets: []structs.ServiceResolverFailoverTarget{{Peer: "cluster-01"}}},
						},
					})),
			},
		}
	}
	addChain := func(t *testing.T, h *peeredUpstreamHarness) {
		h.mustHandle(intentionUpstreams(billing))
		h.mustHandle(chainEvent(t))
		_, ok := h.snap.ConnectProxy.DiscoveryChain[billingUID]
		require.True(t, ok)
	}

	t.Run("chain target and imported upstream share an upstream ID", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)
		addChain(t, h)

		chainRegs := rec.registrations(healthID)
		require.Len(t, chainRegs, 1, "the chain's failover target registers its own peer health watch")
		require.True(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(peerUID))
	})

	t.Run("imported upstream does not cancel the chain watch and survives chain removal", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)
		addChain(t, h)
		chainWatch := rec.registrations(healthID)[0]

		h.mustDeliverImported(imported)
		h.mustDeliverImported(imported)

		require.NoError(t, chainWatch.ctx.Err(),
			"registering the imported upstream must not cancel the chain's watch")
		require.Len(t, rec.registrations(healthID), 2,
			"the imported upstream owns exactly one watch of its own")

		h.mustHandle(intentionUpstreams())
		require.Error(t, chainWatch.ctx.Err())
		require.True(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(peerUID))
		require.Equal(t, 1, rec.active(healthID),
			"the imported upstream still needs a live watch after the chain is removed")

		h.mustDeliverImported()
		require.Equal(t, 0, rec.active(healthID))
		require.False(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(peerUID))
	})

	t.Run("removing the imported upstream keeps the chain watch", func(t *testing.T) {
		rec := newPeeredWatchRecorder()
		h := newPeeredUpstreamHarness(t, rec, "web", nil)

		h.mustDeliverImported(imported)
		importedWatch := rec.registrations(healthID)[0]
		addChain(t, h)
		require.Len(t, rec.registrations(healthID), 2)

		h.mustDeliverImported()
		require.Error(t, importedWatch.ctx.Err(),
			"the imported upstream's watch must stop once it is no longer imported")
		require.True(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(peerUID),
			"the chain still targets this peer upstream")
		require.Equal(t, 1, rec.active(healthID))

		h.mustHandle(intentionUpstreams())
		h.mustDeliverImported()
		require.Equal(t, 0, rec.active(healthID))
		require.False(t, h.snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(peerUID))
	})
}

func TestConnectProxyPeeredUpstreamWatchLifecycle_RecorderSanity(t *testing.T) {
	rec := newPeeredWatchRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	rec.failNext("a", fmt.Errorf("boom"))

	require.Error(t, rec.notify(ctx, nil, "a"))
	require.NoError(t, rec.notify(ctx, nil, "a"))
	require.Len(t, rec.attempts("a"), 2)
	require.Len(t, rec.registrations("a"), 1)
	require.Equal(t, 1, rec.active("a"))

	cancel()
	require.Equal(t, 0, rec.active("a"))
}
