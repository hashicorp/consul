// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

type handlerUpstreams struct {
	handlerState
}

func (s *handlerUpstreams) handleUpdateUpstreams(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	upstreamsSnapshot, err := snap.ToConfigSnapshotUpstreams()

	if err != nil {
		return err
	}

	switch {
	case u.CorrelationID == leafWatchID:
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		upstreamsSnapshot.Leaf = leaf

	case u.CorrelationID == meshConfigEntryID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		if resp.Entry != nil {
			meshConf, ok := resp.Entry.(*structs.MeshConfigEntry)
			if !ok {
				return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
			}
			upstreamsSnapshot.MeshConfig = meshConf
		} else {
			upstreamsSnapshot.MeshConfig = nil
		}
		upstreamsSnapshot.MeshConfigSet = true

	case strings.HasPrefix(u.CorrelationID, "discovery-chain:"):
		resp, ok := u.Result.(*structs.DiscoveryChainResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		uidString := strings.TrimPrefix(u.CorrelationID, "discovery-chain:")
		uid := UpstreamIDFromString(uidString)

		switch snap.Kind {
		case structs.ServiceKindAPIGateway:
			if !snap.APIGateway.UpstreamsSet.hasUpstream(uid) {
				// Discovery chain is not associated with a known explicit or implicit upstream so it is purged/skipped.
				// The associated watch was likely cancelled.
				delete(upstreamsSnapshot.DiscoveryChain, uid)
				s.logger.Trace("discovery-chain watch fired for unknown upstream", "upstream", uid)
				return nil
			}
		case structs.ServiceKindIngressGateway:
			if _, ok := snap.IngressGateway.UpstreamsSet[uid]; !ok {
				// Discovery chain is not associated with a known explicit or implicit upstream so it is purged/skipped.
				// The associated watch was likely cancelled.
				delete(upstreamsSnapshot.DiscoveryChain, uid)
				s.logger.Trace("discovery-chain watch fired for unknown upstream", "upstream", uid)
				return nil
			}

		case structs.ServiceKindConnectProxy:
			explicit := snap.ConnectProxy.UpstreamConfig[uid].HasLocalPortOrSocket()
			implicit := snap.ConnectProxy.IsImplicitUpstream(uid)
			if !implicit && !explicit {
				// Discovery chain is not associated with a known explicit or implicit upstream so it is purged/skipped.
				// The associated watch was likely cancelled.
				delete(upstreamsSnapshot.DiscoveryChain, uid)
				s.logger.Trace("discovery-chain watch fired for unknown upstream", "upstream", uid)
				return nil
			}
		default:
			return fmt.Errorf("discovery-chain watch fired for unsupported kind: %s", snap.Kind)
		}

		upstreamsSnapshot.DiscoveryChain[uid] = resp.Chain

		if err := s.resetWatchesFromChain(ctx, uid, resp.Chain, upstreamsSnapshot); err != nil {
			return err
		}

	case strings.HasPrefix(u.CorrelationID, upstreamPeerWatchIDPrefix):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		uidString := strings.TrimPrefix(u.CorrelationID, upstreamPeerWatchIDPrefix)
		uid := UpstreamIDFromString(uidString)

		s.setPeerEndpoints(upstreamsSnapshot, uid, resp.Nodes)

	case strings.HasPrefix(u.CorrelationID, peerTrustBundleIDPrefix):
		resp, ok := u.Result.(*pbpeering.TrustBundleReadResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		peer := strings.TrimPrefix(u.CorrelationID, peerTrustBundleIDPrefix)
		if resp.Bundle != nil {
			upstreamsSnapshot.UpstreamPeerTrustBundles.Set(peer, resp.Bundle)
		}

	case strings.HasPrefix(u.CorrelationID, "upstream-target:"):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		correlationID := strings.TrimPrefix(u.CorrelationID, "upstream-target:")
		targetID, uidString, ok := removeColonPrefix(correlationID)
		if !ok {
			return fmt.Errorf("invalid correlation id %q", u.CorrelationID)
		}

		uid := UpstreamIDFromString(uidString)

		s.logger.Debug("upstream-target watch fired",
			"correlationID", correlationID,
			"nodes", len(resp.Nodes),
		)
		if _, ok := upstreamsSnapshot.WatchedUpstreamEndpoints[uid]; !ok {
			upstreamsSnapshot.WatchedUpstreamEndpoints[uid] = make(map[string]structs.CheckServiceNodes)
		}
		upstreamsSnapshot.WatchedUpstreamEndpoints[uid][targetID] = resp.Nodes

		// Skip adding passthroughs unless it's a connect sidecar in tproxy mode.
		if s.kind != structs.ServiceKindConnectProxy || s.proxyCfg.Mode != structs.ProxyModeTransparent {
			return nil
		}

		// Clear out this target's existing passthrough upstreams and indices so that they can be repopulated below.
		if _, ok := upstreamsSnapshot.PassthroughUpstreams[uid]; ok {
			for addr := range upstreamsSnapshot.PassthroughUpstreams[uid][targetID] {
				if indexed := upstreamsSnapshot.PassthroughIndices[addr]; indexed.targetID == targetID && indexed.upstreamID == uid {
					delete(upstreamsSnapshot.PassthroughIndices, addr)
				}
			}
			upstreamsSnapshot.PassthroughUpstreams[uid][targetID] = make(map[string]struct{})
		}

		passthroughs := make(map[string]struct{})

		for _, node := range resp.Nodes {
			dialedDirectly := node.Service.Proxy.TransparentProxy.DialedDirectly
			// We must do a manual merge here on the DialedDirectly field, because the service-defaults
			// and proxy-defaults are not automatically merged into the CheckServiceNodes when in
			// agentless mode (because the streaming backend doesn't yet support the MergeCentralConfig field).
			if chain := snap.ConnectProxy.DiscoveryChain[uid]; chain != nil {
				if target := chain.Targets[targetID]; target != nil {
					dialedDirectly = dialedDirectly || target.TransparentProxy.DialedDirectly
				}
			}
			// Skip adding a passthrough for the upstream node if not DialedDirectly.
			if !dialedDirectly {
				continue
			}

			// Make sure to use an external address when crossing partition or DC boundaries.
			isRemote := !snap.Locality.Matches(node.Node.Datacenter, node.Node.PartitionOrDefault())
			// If node is peered it must be remote
			if node.Node.PeerOrEmpty() != "" {
				isRemote = true
			}
			csnIdx, addr, _ := node.BestAddress(isRemote)

			existing := upstreamsSnapshot.PassthroughIndices[addr]
			if existing.idx > csnIdx {
				// The last known instance with this address had a higher index so it takes precedence.
				continue
			}

			// The current instance has a higher Raft index so we ensure the passthrough address is only
			// associated with this upstream target. Older associations are cleaned up as needed.
			delete(upstreamsSnapshot.PassthroughUpstreams[existing.upstreamID][existing.targetID], addr)
			if len(upstreamsSnapshot.PassthroughUpstreams[existing.upstreamID][existing.targetID]) == 0 {
				delete(upstreamsSnapshot.PassthroughUpstreams[existing.upstreamID], existing.targetID)
			}
			if len(upstreamsSnapshot.PassthroughUpstreams[existing.upstreamID]) == 0 {
				delete(upstreamsSnapshot.PassthroughUpstreams, existing.upstreamID)
			}

			upstreamsSnapshot.PassthroughIndices[addr] = indexedTarget{idx: csnIdx, upstreamID: uid, targetID: targetID}
			passthroughs[addr] = struct{}{}
		}
		// Always clear out the existing target passthroughs list so that clusters are cleaned up
		// correctly if no entries are populated.
		upstreamsSnapshot.PassthroughUpstreams[uid] = make(map[string]map[string]struct{})
		if len(passthroughs) > 0 {
			// Add the passthroughs to the target if any were found.
			upstreamsSnapshot.PassthroughUpstreams[uid][targetID] = passthroughs
		}

	case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		correlationID := strings.TrimPrefix(u.CorrelationID, "mesh-gateway:")
		key, uidString, ok := strings.Cut(correlationID, ":")
		if ok {
			// correlationID formatted with an upstreamID
			uid := UpstreamIDFromString(uidString)

			if _, ok = upstreamsSnapshot.WatchedGatewayEndpoints[uid]; !ok {
				upstreamsSnapshot.WatchedGatewayEndpoints[uid] = make(map[string]structs.CheckServiceNodes)
			}
			upstreamsSnapshot.WatchedGatewayEndpoints[uid][key] = resp.Nodes
		} else {
			// event was for local gateways only
			upstreamsSnapshot.WatchedLocalGWEndpoints.Set(key, resp.Nodes)
		}
	default:
		return fmt.Errorf("unknown correlation ID: %s", u.CorrelationID)
	}
	return nil
}

func removeColonPrefix(s string) (string, string, bool) {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return "", "", false
	}
	return s[0:idx], s[idx+1:], true
}

func (s *handlerUpstreams) setPeerEndpoints(upstreamsSnapshot *ConfigSnapshotUpstreams, uid UpstreamID, nodes structs.CheckServiceNodes) {
	filteredNodes := hostnameEndpoints(
		s.logger,
		GatewayKey{ /*empty so it never matches*/ },
		nodes,
	)
	if len(filteredNodes) > 0 {
		if set := upstreamsSnapshot.PeerUpstreamEndpoints.Set(uid, filteredNodes); set {
			upstreamsSnapshot.PeerUpstreamEndpointsUseHostnames[uid] = struct{}{}
		}
	} else {
		if set := upstreamsSnapshot.PeerUpstreamEndpoints.Set(uid, nodes); set {
			delete(upstreamsSnapshot.PeerUpstreamEndpointsUseHostnames, uid)
		}
	}
}

func (s *handlerUpstreams) resetWatchesFromChain(
	ctx context.Context,
	uid UpstreamID,
	chain *structs.CompiledDiscoveryChain,
	snap *ConfigSnapshotUpstreams,
) error {
	s.logger.Trace("resetting watches for discovery chain", "id", uid)
	if chain == nil {
		return fmt.Errorf("not possible to arrive here with no discovery chain")
	}

	// Initialize relevant sub maps.
	if _, ok := snap.WatchedUpstreams[uid]; !ok {
		snap.WatchedUpstreams[uid] = make(map[string]context.CancelFunc)
	}
	if _, ok := snap.WatchedUpstreamEndpoints[uid]; !ok {
		snap.WatchedUpstreamEndpoints[uid] = make(map[string]structs.CheckServiceNodes)
	}
	if _, ok := snap.WatchedGateways[uid]; !ok {
		snap.WatchedGateways[uid] = make(map[string]context.CancelFunc)
	}
	if _, ok := snap.WatchedGatewayEndpoints[uid]; !ok {
		snap.WatchedGatewayEndpoints[uid] = make(map[string]structs.CheckServiceNodes)
	}

	// We could invalidate this selectively based on a hash of the relevant
	// resolver information, but for now just reset anything about this
	// upstream when the chain changes in any way.
	//
	// TODO(rb): content hash based add/remove
	for targetID, cancelFn := range snap.WatchedUpstreams[uid] {
		s.logger.Trace("stopping watch of target",
			"upstream", uid,
			"chain", chain.ServiceName,
			"target", targetID,
		)
		delete(snap.WatchedUpstreams[uid], targetID)
		delete(snap.WatchedUpstreamEndpoints[uid], targetID)
		cancelFn()

		targetUID := NewUpstreamIDFromTargetID(targetID)
		if targetUID.Peer != "" {
			snap.PeerUpstreamEndpoints.CancelWatch(targetUID)
			snap.UpstreamPeerTrustBundles.CancelWatch(targetUID.Peer)
		}
	}

	var (
		watchedChainEndpoints bool
		needGateways          = make(map[string]struct{})
	)

	chainID := chain.ID()
	for _, target := range chain.Targets {
		if target.ID == chainID {
			watchedChainEndpoints = true
		}

		opts := targetWatchOpts{
			upstreamID: uid,
			chainID:    target.ID,
			service:    target.Service,
			filter:     target.Subset.Filter,
			datacenter: target.Datacenter,
			peer:       target.Peer,
			entMeta:    target.GetEnterpriseMetadata(),
		}
		// Peering targets do not set the datacenter field, so we should default it here.
		if opts.datacenter == "" {
			opts.datacenter = s.source.Datacenter
		}

		err := s.watchUpstreamTarget(ctx, snap, opts)
		if err != nil {
			return fmt.Errorf("failed to watch target %q for upstream %q", target.ID, uid)
		}

		// We'll get endpoints from the gateway query, but the health still has
		// to come from the backing service query.
		var gk GatewayKey

		switch target.MeshGateway.Mode {
		case structs.MeshGatewayModeRemote:
			gk = GatewayKey{
				Partition:  target.Partition,
				Datacenter: target.Datacenter,
			}
		case structs.MeshGatewayModeLocal:
			gk = GatewayKey{
				Partition:  s.proxyID.PartitionOrDefault(),
				Datacenter: s.source.Datacenter,
			}
		}
		if s.source.Datacenter != target.Datacenter || s.proxyID.PartitionOrDefault() != target.Partition {
			needGateways[gk.String()] = struct{}{}
		}
		// Register a local gateway watch if any targets are pointing to a peer and require a mode of local.
		if target.Peer != "" && target.MeshGateway.Mode == structs.MeshGatewayModeLocal {
			s.setupWatchForLocalGWEndpoints(ctx, snap)
		}
	}

	// If the discovery chain's targets do not lead to watching all endpoints
	// for the upstream, then create a separate watch for those too.
	// This is needed in transparent mode because if there is some service A that
	// redirects to service B, the dialing proxy needs to associate A's virtual IP
	// with A's discovery chain.
	//
	// Outside of transparent mode we only watch the chain target, B,
	// since A is a virtual service and traffic will not be sent to it.
	if !watchedChainEndpoints && s.proxyCfg.Mode == structs.ProxyModeTransparent {
		chainEntMeta := acl.NewEnterpriseMetaWithPartition(chain.Partition, chain.Namespace)

		opts := targetWatchOpts{
			upstreamID: uid,
			chainID:    chainID,
			service:    chain.ServiceName,
			filter:     "",
			datacenter: chain.Datacenter,
			entMeta:    &chainEntMeta,
		}
		err := s.watchUpstreamTarget(ctx, snap, opts)
		if err != nil {
			return fmt.Errorf("failed to watch target %q for upstream %q", chainID, uid)
		}
	}

	for key := range needGateways {
		if _, ok := snap.WatchedGateways[uid][key]; ok {
			continue
		}
		gwKey := gatewayKeyFromString(key)

		s.logger.Trace("initializing watch of mesh gateway",
			"upstream", uid,
			"chain", chain.ServiceName,
			"datacenter", gwKey.Datacenter,
			"partition", gwKey.Partition,
		)

		ctx, cancel := context.WithCancel(ctx)
		opts := gatewayWatchOpts{
			internalServiceDump: s.dataSources.InternalServiceDump,
			notifyCh:            s.ch,
			source:              *s.source,
			token:               s.token,
			key:                 gwKey,
			upstreamID:          uid,
		}
		err := watchMeshGateway(ctx, opts)
		if err != nil {
			cancel()
			return err
		}

		snap.WatchedGateways[uid][key] = cancel
	}

	for key, cancelFn := range snap.WatchedGateways[uid] {
		if _, ok := needGateways[key]; ok {
			continue
		}
		gwKey := gatewayKeyFromString(key)

		s.logger.Trace("stopping watch of mesh gateway",
			"upstream", uid,
			"chain", chain.ServiceName,
			"datacenter", gwKey.Datacenter,
			"partition", gwKey.Partition,
		)
		delete(snap.WatchedGateways[uid], key)
		delete(snap.WatchedGatewayEndpoints[uid], key)
		cancelFn()
	}

	return nil
}

type targetWatchOpts struct {
	upstreamID UpstreamID
	chainID    string
	service    string
	filter     string
	datacenter string
	peer       string
	entMeta    *acl.EnterpriseMeta
}

func (s *handlerUpstreams) watchUpstreamTarget(ctx context.Context, snap *ConfigSnapshotUpstreams, opts targetWatchOpts) error {
	s.logger.Trace("initializing watch of target",
		"upstream", opts.upstreamID,
		"chain", opts.service,
		"target", opts.chainID,
	)

	uid := opts.upstreamID
	correlationID := "upstream-target:" + opts.chainID + ":" + uid.String()

	if opts.peer != "" {
		uid = NewUpstreamIDFromTargetID(opts.chainID)
		correlationID = upstreamPeerWatchIDPrefix + uid.String()
	}

	// Perform this merge so that a nil EntMeta isn't possible.
	var entMeta acl.EnterpriseMeta
	entMeta.Merge(opts.entMeta)

	ctx, cancel := context.WithCancel(ctx)
	err := s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
		PeerName:   opts.peer,
		Datacenter: opts.datacenter,
		QueryOptions: structs.QueryOptions{
			Token:  s.token,
			Filter: opts.filter,
		},
		ServiceName: opts.service,
		Connect:     true,
		// Note that Identifier doesn't type-prefix for service any more as it's
		// the default and makes metrics and other things much cleaner. It's
		// simpler for us if we have the type to make things unambiguous.
		Source:         *s.source,
		EnterpriseMeta: entMeta,
	}, correlationID, s.ch)

	if err != nil {
		cancel()
		return err
	}
	snap.WatchedUpstreams[opts.upstreamID][opts.chainID] = cancel

	if uid.Peer == "" {
		return nil
	}

	if ok := snap.PeerUpstreamEndpoints.IsWatched(uid); !ok {
		snap.PeerUpstreamEndpoints.InitWatch(uid, cancel)
	}

	// Check whether a watch for this peer exists to avoid duplicates.
	if ok := snap.UpstreamPeerTrustBundles.IsWatched(uid.Peer); !ok {
		peerCtx, cancel := context.WithCancel(ctx)
		if err := s.dataSources.TrustBundle.Notify(peerCtx, &cachetype.TrustBundleReadRequest{
			Request: &pbpeering.TrustBundleReadRequest{
				Name:      uid.Peer,
				Partition: uid.PartitionOrDefault(),
			},
			QueryOptions: structs.QueryOptions{Token: s.token},
		}, peerTrustBundleIDPrefix+uid.Peer, s.ch); err != nil {
			cancel()
			return fmt.Errorf("error while watching trust bundle for peer %q: %w", uid.Peer, err)
		}

		snap.UpstreamPeerTrustBundles.InitWatch(uid.Peer, cancel)
	}

	return nil
}

type discoveryChainWatchOpts struct {
	id          UpstreamID
	name        string
	namespace   string
	partition   string
	datacenter  string
	cfg         reducedUpstreamConfig
	meshGateway structs.MeshGatewayConfig
}

func (s *handlerUpstreams) watchDiscoveryChain(ctx context.Context, snap *ConfigSnapshot, opts discoveryChainWatchOpts) error {
	var watchedDiscoveryChains map[UpstreamID]context.CancelFunc
	switch s.kind {
	case structs.ServiceKindAPIGateway:
		watchedDiscoveryChains = snap.APIGateway.WatchedDiscoveryChains
	case structs.ServiceKindIngressGateway:
		watchedDiscoveryChains = snap.IngressGateway.WatchedDiscoveryChains
	case structs.ServiceKindConnectProxy:
		watchedDiscoveryChains = snap.ConnectProxy.WatchedDiscoveryChains
	default:
		return fmt.Errorf("unsupported kind %s", s.kind)
	}

	if _, ok := watchedDiscoveryChains[opts.id]; ok {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	err := s.dataSources.CompiledDiscoveryChain.Notify(ctx, &structs.DiscoveryChainRequest{
		Datacenter:             s.source.Datacenter,
		QueryOptions:           structs.QueryOptions{Token: s.token},
		Name:                   opts.name,
		EvaluateInDatacenter:   opts.datacenter,
		EvaluateInNamespace:    opts.namespace,
		EvaluateInPartition:    opts.partition,
		OverrideProtocol:       opts.cfg.Protocol,
		OverrideConnectTimeout: opts.cfg.ConnectTimeout(),
		OverrideMeshGateway:    opts.meshGateway,
	}, "discovery-chain:"+opts.id.String(), s.ch)
	if err != nil {
		cancel()
		return err
	}

	watchedDiscoveryChains[opts.id] = cancel
	return nil
}

// reducedUpstreamConfig represents the basic opaque config values that are now
// managed with the discovery chain but for backwards compatibility reasons
// should still affect how the proxy is configured.
//
// The full-blown config is agent/xds.UpstreamConfig
type reducedUpstreamConfig struct {
	Protocol         string `mapstructure:"protocol"`
	ConnectTimeoutMs int    `mapstructure:"connect_timeout_ms"`
}

func (c *reducedUpstreamConfig) ConnectTimeout() time.Duration {
	return time.Duration(c.ConnectTimeoutMs) * time.Millisecond
}

func parseReducedUpstreamConfig(m map[string]interface{}) (reducedUpstreamConfig, error) {
	var cfg reducedUpstreamConfig
	err := mapstructure.WeakDecode(m, &cfg)
	return cfg, err
}

func (s *handlerUpstreams) setupWatchForLocalGWEndpoints(
	ctx context.Context,
	upstreams *ConfigSnapshotUpstreams,
) error {
	gk := GatewayKey{
		Partition:  s.proxyID.PartitionOrDefault(),
		Datacenter: s.source.Datacenter,
	}
	// If the watch is already initialized, do nothing.
	if upstreams.WatchedLocalGWEndpoints.IsWatched(gk.String()) {
		return nil
	}

	opts := gatewayWatchOpts{
		internalServiceDump: s.dataSources.InternalServiceDump,
		notifyCh:            s.ch,
		source:              *s.source,
		token:               s.token,
		key:                 gk,
	}
	if err := watchMeshGateway(ctx, opts); err != nil {
		return fmt.Errorf("error while watching for local mesh gateway: %w", err)
	}
	upstreams.WatchedLocalGWEndpoints.InitWatch(gk.String(), nil)
	return nil
}
