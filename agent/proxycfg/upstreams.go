package proxycfg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
)

type handlerUpstreams struct {
	handlerState
}

func (s *handlerUpstreams) handleUpdateUpstreams(ctx context.Context, u cache.UpdateEvent, snap *ConfigSnapshot) error {
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

	case strings.HasPrefix(u.CorrelationID, "discovery-chain:"):
		resp, ok := u.Result.(*structs.DiscoveryChainResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		svc := strings.TrimPrefix(u.CorrelationID, "discovery-chain:")

		explicit := snap.ConnectProxy.UpstreamConfig[svc].HasLocalPortOrSocket()
		if _, implicit := snap.ConnectProxy.IntentionUpstreams[svc]; !implicit && !explicit {
			// Discovery chain is not associated with a known explicit or implicit upstream so it is skipped.
			// The associated watch was likely cancelled.
			return nil
		}

		upstreamsSnapshot.DiscoveryChain[svc] = resp.Chain

		if err := s.resetWatchesFromChain(ctx, svc, resp.Chain, upstreamsSnapshot); err != nil {
			return err
		}

	case strings.HasPrefix(u.CorrelationID, "upstream-target:"):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		correlationID := strings.TrimPrefix(u.CorrelationID, "upstream-target:")
		targetID, svc, ok := removeColonPrefix(correlationID)
		if !ok {
			return fmt.Errorf("invalid correlation id %q", u.CorrelationID)
		}

		if _, ok := upstreamsSnapshot.WatchedUpstreamEndpoints[svc]; !ok {
			upstreamsSnapshot.WatchedUpstreamEndpoints[svc] = make(map[string]structs.CheckServiceNodes)
		}
		upstreamsSnapshot.WatchedUpstreamEndpoints[svc][targetID] = resp.Nodes

		var passthroughAddrs map[string]ServicePassthroughAddrs

		for _, node := range resp.Nodes {
			if snap.Proxy.Mode == structs.ProxyModeTransparent && node.Service.Proxy.TransparentProxy.DialedDirectly {
				if passthroughAddrs == nil {
					passthroughAddrs = make(map[string]ServicePassthroughAddrs)
				}

				svc := node.Service.CompoundServiceName()

				// Overwrite the name if it's a connect proxy (as opposed to Connect native).
				// We don't reference the proxy name directly for things like SNI, but rather the name
				// of the destination. The enterprise meta of a proxy will always be the same as that of
				// the destination service, so that remains intact.
				if node.Service.Kind == structs.ServiceKindConnectProxy {
					dst := node.Service.Proxy.DestinationServiceName
					if dst == "" {
						dst = node.Service.Proxy.DestinationServiceID
					}
					svc.Name = dst
				}

				sni := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), svc.PartitionOrDefault(), snap.Datacenter, snap.Roots.TrustDomain)

				spiffeID := connect.SpiffeIDService{
					Host:       snap.Roots.TrustDomain,
					Partition:  svc.PartitionOrDefault(),
					Namespace:  svc.NamespaceOrDefault(),
					Datacenter: snap.Datacenter,
					Service:    svc.Name,
				}

				if _, ok := upstreamsSnapshot.PassthroughUpstreams[svc.String()]; !ok {
					upstreamsSnapshot.PassthroughUpstreams[svc.String()] = ServicePassthroughAddrs{
						SNI:      sni,
						SpiffeID: spiffeID,

						// Stored in a set because it's possible for these to be duplicated
						// when the upstream-target is targeted by multiple discovery chains.
						Addrs: make(map[string]struct{}),
					}
				}

				// Make sure to use an external address when crossing partitions.
				isRemote := !structs.EqualPartitions(svc.PartitionOrDefault(), s.proxyID.PartitionOrDefault())
				addr, _ := node.BestAddress(isRemote)

				upstreamsSnapshot.PassthroughUpstreams[svc.String()].Addrs[addr] = struct{}{}
			}
		}

	case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
		resp, ok := u.Result.(*structs.IndexedNodesWithGateways)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		correlationID := strings.TrimPrefix(u.CorrelationID, "mesh-gateway:")
		key, svc, ok := removeColonPrefix(correlationID)
		if !ok {
			return fmt.Errorf("invalid correlation id %q", u.CorrelationID)
		}
		if _, ok = upstreamsSnapshot.WatchedGatewayEndpoints[svc]; !ok {
			upstreamsSnapshot.WatchedGatewayEndpoints[svc] = make(map[string]structs.CheckServiceNodes)
		}
		upstreamsSnapshot.WatchedGatewayEndpoints[svc][key] = resp.Nodes

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
	id string,
	chain *structs.CompiledDiscoveryChain,
	snap *ConfigSnapshotUpstreams,
) error {
	s.logger.Trace("resetting watches for discovery chain", "id", id)
	if chain == nil {
		return fmt.Errorf("not possible to arrive here with no discovery chain")
	}

	// Initialize relevant sub maps.
	if _, ok := snap.WatchedUpstreams[id]; !ok {
		snap.WatchedUpstreams[id] = make(map[string]context.CancelFunc)
	}
	if _, ok := snap.WatchedUpstreamEndpoints[id]; !ok {
		snap.WatchedUpstreamEndpoints[id] = make(map[string]structs.CheckServiceNodes)
	}
	if _, ok := snap.WatchedGateways[id]; !ok {
		snap.WatchedGateways[id] = make(map[string]context.CancelFunc)
	}
	if _, ok := snap.WatchedGatewayEndpoints[id]; !ok {
		snap.WatchedGatewayEndpoints[id] = make(map[string]structs.CheckServiceNodes)
	}

	// We could invalidate this selectively based on a hash of the relevant
	// resolver information, but for now just reset anything about this
	// upstream when the chain changes in any way.
	//
	// TODO(rb): content hash based add/remove
	for targetID, cancelFn := range snap.WatchedUpstreams[id] {
		s.logger.Trace("stopping watch of target",
			"upstream", id,
			"chain", chain.ServiceName,
			"target", targetID,
		)
		delete(snap.WatchedUpstreams[id], targetID)
		delete(snap.WatchedUpstreamEndpoints[id], targetID)
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
			upstreamID: id,
			chainID:    target.ID,
			service:    target.Service,
			filter:     target.Subset.Filter,
			datacenter: target.Datacenter,
			entMeta:    target.GetEnterpriseMetadata(),
		}
		err := s.watchUpstreamTarget(ctx, snap, opts)
		if err != nil {
			return fmt.Errorf("failed to watch target %q for upstream %q", target.ID, id)
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
		chainEntMeta := structs.NewEnterpriseMetaWithPartition(chain.Partition, chain.Namespace)

		opts := targetWatchOpts{
			upstreamID: id,
			chainID:    chainID,
			service:    chain.ServiceName,
			filter:     "",
			datacenter: chain.Datacenter,
			entMeta:    &chainEntMeta,
		}
		err := s.watchUpstreamTarget(ctx, snap, opts)
		if err != nil {
			return fmt.Errorf("failed to watch target %q for upstream %q", chainID, id)
		}
	}

	for key := range needGateways {
		if _, ok := snap.WatchedGateways[id][key]; ok {
			continue
		}
		gwKey := gatewayKeyFromString(key)

		s.logger.Trace("initializing watch of mesh gateway",
			"upstream", id,
			"chain", chain.ServiceName,
			"datacenter", gwKey.Datacenter,
			"partition", gwKey.Partition,
		)

		ctx, cancel := context.WithCancel(ctx)
		opts := gatewayWatchOpts{
			notifier:   s.cache,
			notifyCh:   s.ch,
			source:     *s.source,
			token:      s.token,
			key:        gwKey,
			upstreamID: id,
		}
		err := watchMeshGateway(ctx, opts)
		if err != nil {
			cancel()
			return err
		}

		snap.WatchedGateways[id][key] = cancel
	}

	for key, cancelFn := range snap.WatchedGateways[id] {
		if _, ok := needGateways[key]; ok {
			continue
		}
		gwKey := gatewayKeyFromString(key)

		s.logger.Trace("stopping watch of mesh gateway",
			"upstream", id,
			"chain", chain.ServiceName,
			"datacenter", gwKey.Datacenter,
			"partition", gwKey.Partition,
		)
		delete(snap.WatchedGateways[id], key)
		delete(snap.WatchedGatewayEndpoints[id], key)
		cancelFn()
	}

	return nil
}

type targetWatchOpts struct {
	upstreamID string
	chainID    string
	service    string
	filter     string
	datacenter string
	entMeta    *structs.EnterpriseMeta
}

func (s *handlerUpstreams) watchUpstreamTarget(ctx context.Context, snap *ConfigSnapshotUpstreams, opts targetWatchOpts) error {
	s.logger.Trace("initializing watch of target",
		"upstream", opts.upstreamID,
		"chain", opts.service,
		"target", opts.chainID,
	)

	var finalMeta structs.EnterpriseMeta
	finalMeta.Merge(opts.entMeta)

	correlationID := "upstream-target:" + opts.chainID + ":" + opts.upstreamID

	ctx, cancel := context.WithCancel(ctx)
	err := s.health.Notify(ctx, structs.ServiceSpecificRequest{
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
	id          string
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
	err := s.cache.Notify(ctx, cachetype.CompiledDiscoveryChainName, &structs.DiscoveryChainRequest{
		Datacenter:             s.source.Datacenter,
		QueryOptions:           structs.QueryOptions{Token: s.token},
		Name:                   opts.name,
		EvaluateInDatacenter:   opts.datacenter,
		EvaluateInNamespace:    opts.namespace,
		EvaluateInPartition:    opts.partition,
		OverrideProtocol:       opts.cfg.Protocol,
		OverrideConnectTimeout: opts.cfg.ConnectTimeout(),
		OverrideMeshGateway:    opts.meshGateway,
	}, "discovery-chain:"+opts.id, s.ch)
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
