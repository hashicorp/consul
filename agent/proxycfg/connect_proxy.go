package proxycfg

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
)

type handlerConnectProxy struct {
	handlerState
}

// initialize sets up the watches needed based on current proxy registration
// state.
func (s *handlerConnectProxy) initialize(ctx context.Context) (ConfigSnapshot, error) {
	snap := newConfigSnapshotFromServiceInstance(s.serviceInstance, s.stateConfig)
	snap.ConnectProxy.DiscoveryChain = make(map[UpstreamID]*structs.CompiledDiscoveryChain)
	snap.ConnectProxy.WatchedDiscoveryChains = make(map[UpstreamID]context.CancelFunc)
	snap.ConnectProxy.WatchedUpstreams = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.ConnectProxy.WatchedUpstreamEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.ConnectProxy.WatchedGateways = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.ConnectProxy.WatchedGatewayEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.ConnectProxy.WatchedServiceChecks = make(map[structs.ServiceID][]structs.CheckType)
	snap.ConnectProxy.PreparedQueryEndpoints = make(map[UpstreamID]structs.CheckServiceNodes)
	snap.ConnectProxy.UpstreamConfig = make(map[UpstreamID]*structs.Upstream)
	snap.ConnectProxy.PassthroughUpstreams = make(map[UpstreamID]map[string]map[string]struct{})
	snap.ConnectProxy.PassthroughIndices = make(map[string]indexedTarget)

	// Watch for root changes
	err := s.cache.Notify(ctx, cachetype.ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch the leaf cert
	err = s.cache.Notify(ctx, cachetype.ConnectCALeafName, &cachetype.ConnectCALeafRequest{
		Datacenter:     s.source.Datacenter,
		Token:          s.token,
		Service:        s.proxyCfg.DestinationServiceName,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, leafWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch for intention updates
	err = s.cache.Notify(ctx, cachetype.IntentionMatchName, &structs.IntentionQueryRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: s.proxyID.NamespaceOrDefault(),
					Partition: s.proxyID.PartitionOrDefault(),
					Name:      s.proxyCfg.DestinationServiceName,
				},
			},
		},
	}, intentionsWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch for service check updates
	err = s.cache.Notify(ctx, cachetype.ServiceHTTPChecksName, &cachetype.ServiceHTTPChecksRequest{
		ServiceID:      s.proxyCfg.DestinationServiceID,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, svcChecksWatchIDPrefix+structs.ServiceIDString(s.proxyCfg.DestinationServiceID, &s.proxyID.EnterpriseMeta), s.ch)
	if err != nil {
		return snap, err
	}

	if s.proxyCfg.Mode == structs.ProxyModeTransparent {
		// When in transparent proxy we will infer upstreams from intentions with this source
		err := s.cache.Notify(ctx, cachetype.IntentionUpstreamsName, &structs.ServiceSpecificRequest{
			Datacenter:     s.source.Datacenter,
			QueryOptions:   structs.QueryOptions{Token: s.token},
			ServiceName:    s.proxyCfg.DestinationServiceName,
			EnterpriseMeta: s.proxyID.EnterpriseMeta,
		}, intentionUpstreamsID, s.ch)
		if err != nil {
			return snap, err
		}

		err = s.cache.Notify(ctx, cachetype.ConfigEntryName, &structs.ConfigEntryQuery{
			Kind:           structs.MeshConfig,
			Name:           structs.MeshConfigMesh,
			Datacenter:     s.source.Datacenter,
			QueryOptions:   structs.QueryOptions{Token: s.token},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(s.proxyID.PartitionOrDefault()),
		}, meshConfigEntryID, s.ch)
		if err != nil {
			return snap, err
		}
	}

	// Watch for updates to service endpoints for all upstreams
	for i := range s.proxyCfg.Upstreams {
		u := s.proxyCfg.Upstreams[i]

		uid := NewUpstreamID(&u)

		// Store defaults keyed under wildcard so they can be applied to centrally configured upstreams
		if u.DestinationName == structs.WildcardSpecifier {
			snap.ConnectProxy.UpstreamConfig[uid] = &u
			continue
		}

		// This can be true if the upstream is a synthetic entry populated from centralized upstream config.
		// Watches should not be created for them.
		if u.CentrallyConfigured {
			continue
		}
		snap.ConnectProxy.UpstreamConfig[uid] = &u

		dc := s.source.Datacenter
		if u.Datacenter != "" {
			dc = u.Datacenter
		}
		if s.proxyCfg.Mode == structs.ProxyModeTransparent && (dc == "" || dc == s.source.Datacenter) {
			// In transparent proxy mode, watches for upstreams in the local DC are handled by the IntentionUpstreams watch.
			continue
		}

		// Default the partition and namespace to the namespace of this proxy service.
		partition := s.proxyID.PartitionOrDefault()
		if u.DestinationPartition != "" {
			partition = u.DestinationPartition
		}
		ns := s.proxyID.NamespaceOrDefault()
		if u.DestinationNamespace != "" {
			ns = u.DestinationNamespace
		}

		cfg, err := parseReducedUpstreamConfig(u.Config)
		if err != nil {
			// Don't hard fail on a config typo, just warn. We'll fall back on
			// the plain discovery chain if there is an error so it's safe to
			// continue.
			s.logger.Warn("failed to parse upstream config",
				"upstream", uid.String(),
				"error", err,
			)
		}

		switch u.DestinationType {
		case structs.UpstreamDestTypePreparedQuery:
			err = s.cache.Notify(ctx, cachetype.PreparedQueryName, &structs.PreparedQueryExecuteRequest{
				Datacenter:    dc,
				QueryOptions:  structs.QueryOptions{Token: s.token, MaxAge: defaultPreparedQueryPollInterval},
				QueryIDOrName: u.DestinationName,
				Connect:       true,
				Source:        *s.source,
			}, "upstream:"+uid.String(), s.ch)
			if err != nil {
				return snap, err
			}

		case structs.UpstreamDestTypeService:
			fallthrough

		case "":
			err = s.cache.Notify(ctx, cachetype.CompiledDiscoveryChainName, &structs.DiscoveryChainRequest{
				Datacenter:             s.source.Datacenter,
				QueryOptions:           structs.QueryOptions{Token: s.token},
				Name:                   u.DestinationName,
				EvaluateInDatacenter:   dc,
				EvaluateInNamespace:    ns,
				EvaluateInPartition:    partition,
				OverrideMeshGateway:    s.proxyCfg.MeshGateway.OverlayWith(u.MeshGateway),
				OverrideProtocol:       cfg.Protocol,
				OverrideConnectTimeout: cfg.ConnectTimeout(),
			}, "discovery-chain:"+uid.String(), s.ch)
			if err != nil {
				return snap, fmt.Errorf("failed to watch discovery chain for %s: %v", uid.String(), err)
			}

		default:
			return snap, fmt.Errorf("unknown upstream type: %q", u.DestinationType)
		}
	}

	return snap, nil
}

func (s *handlerConnectProxy) handleUpdate(ctx context.Context, u cache.UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	switch {
	case u.CorrelationID == rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.Roots = roots
	case u.CorrelationID == intentionsWatchID:
		resp, ok := u.Result.(*structs.IndexedIntentionMatches)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if len(resp.Matches) > 0 {
			// RPC supports matching multiple services at once but we only ever
			// query with the one service we represent currently so just pick
			// the one result set up.
			snap.ConnectProxy.Intentions = resp.Matches[0]
		}
		snap.ConnectProxy.IntentionsSet = true

	case u.CorrelationID == intentionUpstreamsID:
		resp, ok := u.Result.(*structs.IndexedServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response %T", u.Result)
		}

		seenUpstreams := make(map[UpstreamID]struct{})
		for _, svc := range resp.Services {
			uid := NewUpstreamIDFromServiceName(svc)

			seenUpstreams[uid] = struct{}{}

			cfgMap := make(map[string]interface{})
			u, ok := snap.ConnectProxy.UpstreamConfig[uid]
			if ok {
				cfgMap = u.Config
			} else {
				// Use the centralized upstream defaults if they exist and there isn't specific configuration for this upstream
				// This is only relevant to upstreams from intentions because for explicit upstreams the defaulting is handled
				// by the ResolveServiceConfig endpoint.
				wildcardSID := structs.NewServiceID(structs.WildcardSpecifier, s.proxyID.WithWildcardNamespace())
				wildcardUID := NewUpstreamIDFromServiceID(wildcardSID)
				defaults, ok := snap.ConnectProxy.UpstreamConfig[wildcardUID]
				if ok {
					u = defaults
					cfgMap = defaults.Config
					snap.ConnectProxy.UpstreamConfig[uid] = defaults
				}
			}

			cfg, err := parseReducedUpstreamConfig(cfgMap)
			if err != nil {
				// Don't hard fail on a config typo, just warn. We'll fall back on
				// the plain discovery chain if there is an error so it's safe to
				// continue.
				s.logger.Warn("failed to parse upstream config",
					"upstream", uid,
					"error", err,
				)
			}

			meshGateway := s.proxyCfg.MeshGateway
			if u != nil {
				meshGateway = meshGateway.OverlayWith(u.MeshGateway)
			}
			watchOpts := discoveryChainWatchOpts{
				id:          NewUpstreamIDFromServiceName(svc),
				name:        svc.Name,
				namespace:   svc.NamespaceOrDefault(),
				partition:   svc.PartitionOrDefault(),
				datacenter:  s.source.Datacenter,
				cfg:         cfg,
				meshGateway: meshGateway,
			}
			up := &handlerUpstreams{handlerState: s.handlerState}
			err = up.watchDiscoveryChain(ctx, snap, watchOpts)
			if err != nil {
				return fmt.Errorf("failed to watch discovery chain for %s: %v", uid, err)
			}
		}
		snap.ConnectProxy.IntentionUpstreams = seenUpstreams

		// Clean up data from services that were not in the update
		for uid, targets := range snap.ConnectProxy.WatchedUpstreams {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && upstream.Datacenter != "" && upstream.Datacenter != s.source.Datacenter {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				for _, cancelFn := range targets {
					cancelFn()
				}
				delete(snap.ConnectProxy.WatchedUpstreams, uid)
			}
		}
		for uid := range snap.ConnectProxy.WatchedUpstreamEndpoints {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && upstream.Datacenter != "" && upstream.Datacenter != s.source.Datacenter {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				delete(snap.ConnectProxy.WatchedUpstreamEndpoints, uid)
			}
		}
		for uid, cancelMap := range snap.ConnectProxy.WatchedGateways {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && upstream.Datacenter != "" && upstream.Datacenter != s.source.Datacenter {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				for _, cancelFn := range cancelMap {
					cancelFn()
				}
				delete(snap.ConnectProxy.WatchedGateways, uid)
			}
		}
		for uid := range snap.ConnectProxy.WatchedGatewayEndpoints {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && upstream.Datacenter != "" && upstream.Datacenter != s.source.Datacenter {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				delete(snap.ConnectProxy.WatchedGatewayEndpoints, uid)
			}
		}
		for uid, cancelFn := range snap.ConnectProxy.WatchedDiscoveryChains {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && upstream.Datacenter != "" && upstream.Datacenter != s.source.Datacenter {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				cancelFn()
				delete(snap.ConnectProxy.WatchedDiscoveryChains, uid)
			}
		}
		for uid := range snap.ConnectProxy.PassthroughUpstreams {
			if _, ok := seenUpstreams[uid]; !ok {
				delete(snap.ConnectProxy.PassthroughUpstreams, uid)
			}
		}
		for addr, indexed := range snap.ConnectProxy.PassthroughIndices {
			if _, ok := seenUpstreams[indexed.upstreamID]; !ok {
				delete(snap.ConnectProxy.PassthroughIndices, addr)
			}
		}

		// These entries are intentionally handled separately from the WatchedDiscoveryChains above.
		// There have been situations where a discovery watch was cancelled, then fired.
		// That update event then re-populated the DiscoveryChain map entry, which wouldn't get cleaned up
		// since there was no known watch for it.
		for uid := range snap.ConnectProxy.DiscoveryChain {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && upstream.Datacenter != "" && upstream.Datacenter != s.source.Datacenter {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				delete(snap.ConnectProxy.DiscoveryChain, uid)
			}
		}

	case strings.HasPrefix(u.CorrelationID, "upstream:"+preparedQueryIDPrefix):
		resp, ok := u.Result.(*structs.PreparedQueryExecuteResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		pq := strings.TrimPrefix(u.CorrelationID, "upstream:")
		uid := UpstreamIDFromString(pq)
		snap.ConnectProxy.PreparedQueryEndpoints[uid] = resp.Nodes

	case strings.HasPrefix(u.CorrelationID, svcChecksWatchIDPrefix):
		resp, ok := u.Result.([]structs.CheckType)
		if !ok {
			return fmt.Errorf("invalid type for service checks response: %T, want: []structs.CheckType", u.Result)
		}
		svcID := structs.ServiceIDFromString(strings.TrimPrefix(u.CorrelationID, svcChecksWatchIDPrefix))
		snap.ConnectProxy.WatchedServiceChecks[svcID] = resp

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
			snap.ConnectProxy.MeshConfig = meshConf
		} else {
			snap.ConnectProxy.MeshConfig = nil
		}
		snap.ConnectProxy.MeshConfigSet = true

	default:
		return (*handlerUpstreams)(s).handleUpdateUpstreams(ctx, u, snap)
	}
	return nil
}
