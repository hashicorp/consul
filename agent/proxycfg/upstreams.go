package proxycfg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type handlerUpstreams struct {
	handlerState
}

func (s *handlerUpstreams) handleUpdateUpstreams(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	upstreamsSnapshot := &snap.ConnectProxy.ConfigSnapshotUpstreams
	if snap.Kind == structs.ServiceKindIngressGateway {
		upstreamsSnapshot = &snap.IngressGateway.ConfigSnapshotUpstreams
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
			if _, implicit := snap.ConnectProxy.IntentionUpstreams[uid]; !implicit && !explicit {
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

		filteredNodes := hostnameEndpoints(
			s.logger,
			GatewayKey{ /*empty so it never matches*/ },
			resp.Nodes,
		)
		if len(filteredNodes) > 0 {
			upstreamsSnapshot.PeerUpstreamEndpoints[uid] = filteredNodes
			upstreamsSnapshot.PeerUpstreamEndpointsUseHostnames[uid] = struct{}{}
		} else {
			upstreamsSnapshot.PeerUpstreamEndpoints[uid] = resp.Nodes
			delete(upstreamsSnapshot.PeerUpstreamEndpointsUseHostnames, uid)
		}

		if s.kind != structs.ServiceKindConnectProxy || s.proxyCfg.Mode != structs.ProxyModeTransparent {
			return nil
		}

		s.logger.Warn("skipping transparent proxy update for peered upstream")

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

		if _, ok := upstreamsSnapshot.WatchedUpstreamEndpoints[uid]; !ok {
			upstreamsSnapshot.WatchedUpstreamEndpoints[uid] = make(map[string]structs.CheckServiceNodes)
		}
		upstreamsSnapshot.WatchedUpstreamEndpoints[uid][targetID] = resp.Nodes

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
			if !node.Service.Proxy.TransparentProxy.DialedDirectly {
				continue
			}

			// Make sure to use an external address when crossing partition or DC boundaries.
			isRemote := !snap.Locality.Matches(node.Node.Datacenter, node.Node.PartitionOrDefault())
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
		if len(passthroughs) > 0 {
			upstreamsSnapshot.PassthroughUpstreams[uid] = map[string]map[string]struct{}{
				targetID: passthroughs,
			}
		}

	case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
		resp, ok := u.Result.(*structs.IndexedNodesWithGateways)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		correlationID := strings.TrimPrefix(u.CorrelationID, "mesh-gateway:")
		key, uidString, ok := removeColonPrefix(correlationID)
		if !ok {
			return fmt.Errorf("invalid correlation id %q", u.CorrelationID)
		}
		uid := UpstreamIDFromString(uidString)

		if _, ok = upstreamsSnapshot.WatchedGatewayEndpoints[uid]; !ok {
			upstreamsSnapshot.WatchedGatewayEndpoints[uid] = make(map[string]structs.CheckServiceNodes)
		}
		upstreamsSnapshot.WatchedGatewayEndpoints[uid][key] = resp.Nodes

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
			entMeta:    target.GetEnterpriseMetadata(),
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
				Partition:  s.source.NodePartitionOrDefault(),
				Datacenter: s.source.Datacenter,
			}
		}
		if s.source.Datacenter != target.Datacenter || s.proxyID.PartitionOrDefault() != target.Partition {
			needGateways[gk.String()] = struct{}{}
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
	entMeta    *acl.EnterpriseMeta
}

func (s *handlerUpstreams) watchUpstreamTarget(ctx context.Context, snap *ConfigSnapshotUpstreams, opts targetWatchOpts) error {
	s.logger.Trace("initializing watch of target",
		"upstream", opts.upstreamID,
		"chain", opts.service,
		"target", opts.chainID,
	)

	var finalMeta acl.EnterpriseMeta
	finalMeta.Merge(opts.entMeta)

	correlationID := "upstream-target:" + opts.chainID + ":" + opts.upstreamID.String()

	ctx, cancel := context.WithCancel(ctx)
	err := s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
		PeerName:   opts.upstreamID.Peer,
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
		EnterpriseMeta: finalMeta,
	}, correlationID, s.ch)

	if err != nil {
		cancel()
		return err
	}
	snap.WatchedUpstreams[opts.upstreamID][opts.chainID] = cancel

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
	if _, ok := snap.ConnectProxy.WatchedDiscoveryChains[opts.id]; ok {
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

	switch s.kind {
	case structs.ServiceKindIngressGateway:
		snap.IngressGateway.WatchedDiscoveryChains[opts.id] = cancel
	case structs.ServiceKindConnectProxy:
		snap.ConnectProxy.WatchedDiscoveryChains[opts.id] = cancel
	default:
		cancel()
		return fmt.Errorf("unsupported kind %s", s.kind)
	}

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
