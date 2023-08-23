package proxycfg

import (
	"context"
	"fmt"
	"strings"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
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
	snap.ConnectProxy.UpstreamPeerTrustBundles = watch.NewMap[string, *pbpeering.PeeringTrustBundle]()
	snap.ConnectProxy.WatchedGateways = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.ConnectProxy.WatchedGatewayEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.ConnectProxy.WatchedServiceChecks = make(map[structs.ServiceID][]structs.CheckType)
	snap.ConnectProxy.PreparedQueryEndpoints = make(map[UpstreamID]structs.CheckServiceNodes)
	snap.ConnectProxy.DestinationsUpstream = watch.NewMap[UpstreamID, *structs.ServiceConfigEntry]()
	snap.ConnectProxy.UpstreamConfig = make(map[UpstreamID]*structs.Upstream)
	snap.ConnectProxy.PassthroughUpstreams = make(map[UpstreamID]map[string]map[string]struct{})
	snap.ConnectProxy.PassthroughIndices = make(map[string]indexedTarget)
	snap.ConnectProxy.PeerUpstreamEndpoints = watch.NewMap[UpstreamID, structs.CheckServiceNodes]()
	snap.ConnectProxy.DestinationGateways = watch.NewMap[UpstreamID, structs.CheckServiceNodes]()
	snap.ConnectProxy.PeerUpstreamEndpointsUseHostnames = make(map[UpstreamID]struct{})

	// Watch for root changes
	err := s.dataSources.CARoots.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	if s.peeringEnabled {
		err = s.dataSources.TrustBundleList.Notify(ctx, &cachetype.TrustBundleListRequest{
			Request: &pbpeering.TrustBundleListByServiceRequest{
				ServiceName: s.proxyCfg.DestinationServiceName,
				Namespace:   s.proxyID.NamespaceOrDefault(),
				Partition:   s.proxyID.PartitionOrDefault(),
			},
			QueryOptions: structs.QueryOptions{Token: s.token},
		}, peeringTrustBundlesWatchID, s.ch)
		if err != nil {
			return snap, err
		}
	}

	// Watch the leaf cert
	err = s.dataSources.LeafCertificate.Notify(ctx, &cachetype.ConnectCALeafRequest{
		Datacenter:     s.source.Datacenter,
		Token:          s.token,
		Service:        s.proxyCfg.DestinationServiceName,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, leafWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch for intention updates
	err = s.dataSources.Intentions.Notify(ctx, &structs.ServiceSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
		ServiceName:    s.proxyCfg.DestinationServiceName,
	}, intentionsWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Get information about the entire service mesh.
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.MeshConfig,
		Name:           structs.MeshConfigMesh,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(s.proxyID.PartitionOrDefault()),
	}, meshConfigEntryID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch for service check updates
	err = s.dataSources.HTTPChecks.Notify(ctx, &cachetype.ServiceHTTPChecksRequest{
		ServiceID:      s.proxyCfg.DestinationServiceID,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, svcChecksWatchIDPrefix+structs.ServiceIDString(s.proxyCfg.DestinationServiceID, &s.proxyID.EnterpriseMeta), s.ch)
	if err != nil {
		return snap, err
	}

	if s.proxyCfg.Mode == structs.ProxyModeTransparent {
		// When in transparent proxy we will infer upstreams from intentions with this source
		err := s.dataSources.IntentionUpstreams.Notify(ctx, &structs.ServiceSpecificRequest{
			Datacenter:     s.source.Datacenter,
			QueryOptions:   structs.QueryOptions{Token: s.token},
			ServiceName:    s.proxyCfg.DestinationServiceName,
			EnterpriseMeta: s.proxyID.EnterpriseMeta,
		}, intentionUpstreamsID, s.ch)
		if err != nil {
			return snap, err
		}
		if s.peeringEnabled {
			err = s.dataSources.PeeredUpstreams.Notify(ctx, &structs.PartitionSpecificRequest{
				QueryOptions:   structs.QueryOptions{Token: s.token},
				Datacenter:     s.source.Datacenter,
				EnterpriseMeta: s.proxyID.EnterpriseMeta,
			}, peeredUpstreamsID, s.ch)
			if err != nil {
				return snap, err
			}
		}
		// We also infer upstreams from destinations (egress points)
		err = s.dataSources.IntentionUpstreamsDestination.Notify(ctx, &structs.ServiceSpecificRequest{
			Datacenter:     s.source.Datacenter,
			QueryOptions:   structs.QueryOptions{Token: s.token},
			ServiceName:    s.proxyCfg.DestinationServiceName,
			EnterpriseMeta: s.proxyID.EnterpriseMeta,
		}, intentionUpstreamsDestinationID, s.ch)
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

		snap.ConnectProxy.UpstreamConfig[uid] = &u
		// This can be true if the upstream is a synthetic entry populated from centralized upstream config.
		// Watches should not be created for them.
		if u.CentrallyConfigured {
			continue
		}

		dc := s.source.Datacenter
		if u.Datacenter != "" {
			dc = u.Datacenter
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
			err = s.dataSources.PreparedQuery.Notify(ctx, &structs.PreparedQueryExecuteRequest{
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
			if u.DestinationPeer != "" && s.peeringEnabled {
				// NOTE: An upstream that points to a peer by definition will
				// only ever watch a single catalog query, so a map key of just
				// "UID" is sufficient to cover the peer data watches here.

				s.logger.Trace("initializing watch of peered upstream", "upstream", uid)

				snap.ConnectProxy.PeerUpstreamEndpoints.InitWatch(uid, nil)
				err := s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
					PeerName:   uid.Peer,
					Datacenter: dc,
					QueryOptions: structs.QueryOptions{
						Token: s.token,
					},
					ServiceName: u.DestinationName,
					Connect:     true,
					// Note that Identifier doesn't type-prefix for service any more as it's
					// the default and makes metrics and other things much cleaner. It's
					// simpler for us if we have the type to make things unambiguous.
					Source:         *s.source,
					EnterpriseMeta: uid.EnterpriseMeta,
				}, upstreamPeerWatchIDPrefix+uid.String(), s.ch)
				if err != nil {
					return snap, err
				}

				// Check whether a watch for this peer exists to avoid duplicates.
				if ok := snap.ConnectProxy.UpstreamPeerTrustBundles.IsWatched(uid.Peer); !ok {
					peerCtx, cancel := context.WithCancel(ctx)
					if err := s.dataSources.TrustBundle.Notify(peerCtx, &cachetype.TrustBundleReadRequest{
						Request: &pbpeering.TrustBundleReadRequest{
							Name:      uid.Peer,
							Partition: uid.PartitionOrDefault(),
						},
						QueryOptions: structs.QueryOptions{Token: s.token},
					}, peerTrustBundleIDPrefix+uid.Peer, s.ch); err != nil {
						cancel()
						return snap, fmt.Errorf("error while watching trust bundle for peer %q: %w", uid.Peer, err)
					}

					snap.ConnectProxy.UpstreamPeerTrustBundles.InitWatch(uid.Peer, cancel)
				}
				continue
			}

			err = s.dataSources.CompiledDiscoveryChain.Notify(ctx, &structs.DiscoveryChainRequest{
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

func (s *handlerConnectProxy) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
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

	case strings.HasPrefix(u.CorrelationID, peerTrustBundleIDPrefix):
		resp, ok := u.Result.(*pbpeering.TrustBundleReadResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		peer := strings.TrimPrefix(u.CorrelationID, peerTrustBundleIDPrefix)
		if resp.Bundle != nil {
			snap.ConnectProxy.UpstreamPeerTrustBundles.Set(peer, resp.Bundle)
		}

	case u.CorrelationID == peeringTrustBundlesWatchID && s.peeringEnabled:
		resp, ok := u.Result.(*pbpeering.TrustBundleListByServiceResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if len(resp.Bundles) > 0 {
			snap.ConnectProxy.InboundPeerTrustBundles = resp.Bundles
		}
		snap.ConnectProxy.InboundPeerTrustBundlesSet = true

	case u.CorrelationID == intentionsWatchID:
		resp, ok := u.Result.(structs.Intentions)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.ConnectProxy.Intentions = resp
		snap.ConnectProxy.IntentionsSet = true

	case u.CorrelationID == peeredUpstreamsID && s.peeringEnabled:
		resp, ok := u.Result.(*structs.IndexedPeeredServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response %T", u.Result)
		}

		seenUpstreams := make(map[UpstreamID]struct{})
		for _, psn := range resp.Services {
			uid := NewUpstreamIDFromPeeredServiceName(psn)

			if _, ok := seenUpstreams[uid]; ok {
				continue
			}
			seenUpstreams[uid] = struct{}{}

			s.logger.Trace("initializing watch of peered upstream", "upstream", uid)

			hctx, hcancel := context.WithCancel(ctx)
			err := s.dataSources.Health.Notify(hctx, &structs.ServiceSpecificRequest{
				PeerName:   uid.Peer,
				Datacenter: s.source.Datacenter,
				QueryOptions: structs.QueryOptions{
					Token: s.token,
				},
				ServiceName: psn.ServiceName.Name,
				Connect:     true,
				// Note that Identifier doesn't type-prefix for service any more as it's
				// the default and makes metrics and other things much cleaner. It's
				// simpler for us if we have the type to make things unambiguous.
				Source:         *s.source,
				EnterpriseMeta: uid.EnterpriseMeta,
			}, upstreamPeerWatchIDPrefix+uid.String(), s.ch)
			if err != nil {
				hcancel()
				return fmt.Errorf("failed to watch health for %s: %v", uid, err)
			}
			snap.ConnectProxy.PeerUpstreamEndpoints.InitWatch(uid, hcancel)

			// Check whether a watch for this peer exists to avoid duplicates.
			if ok := snap.ConnectProxy.UpstreamPeerTrustBundles.IsWatched(uid.Peer); !ok {
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

				snap.ConnectProxy.UpstreamPeerTrustBundles.InitWatch(uid.Peer, cancel)
			}
		}
		snap.ConnectProxy.PeeredUpstreams = seenUpstreams

		//
		// Clean up data
		//

		validPeerNames := make(map[string]struct{})

		// Iterate through all known endpoints and remove references to upstream IDs that weren't in the update
		snap.ConnectProxy.PeerUpstreamEndpoints.ForEachKey(func(uid UpstreamID) bool {
			// Peered upstream is explicitly defined in upstream config
			if _, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok {
				validPeerNames[uid.Peer] = struct{}{}
				return true
			}
			// Peered upstream came from dynamic source of imported services
			if _, ok := seenUpstreams[uid]; ok {
				validPeerNames[uid.Peer] = struct{}{}
				return true
			}
			snap.ConnectProxy.PeerUpstreamEndpoints.CancelWatch(uid)
			return true
		})

		// Iterate through all known trust bundles and remove references to any unseen peer names
		snap.ConnectProxy.UpstreamPeerTrustBundles.ForEachKey(func(peerName PeerName) bool {
			if _, ok := validPeerNames[peerName]; !ok {
				snap.ConnectProxy.UpstreamPeerTrustBundles.CancelWatch(peerName)
			}
			return true
		})

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
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && !upstream.CentrallyConfigured {
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
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && !upstream.CentrallyConfigured {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				delete(snap.ConnectProxy.WatchedUpstreamEndpoints, uid)
			}
		}
		for uid, cancelMap := range snap.ConnectProxy.WatchedGateways {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && !upstream.CentrallyConfigured {
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
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && !upstream.CentrallyConfigured {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				delete(snap.ConnectProxy.WatchedGatewayEndpoints, uid)
			}
		}
		for uid, cancelFn := range snap.ConnectProxy.WatchedDiscoveryChains {
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && !upstream.CentrallyConfigured {
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
			if upstream, ok := snap.ConnectProxy.UpstreamConfig[uid]; ok && !upstream.CentrallyConfigured {
				continue
			}
			if _, ok := seenUpstreams[uid]; !ok {
				delete(snap.ConnectProxy.DiscoveryChain, uid)
			}
		}
	case u.CorrelationID == intentionUpstreamsDestinationID:
		resp, ok := u.Result.(*structs.IndexedServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response %T", u.Result)
		}
		seenUpstreams := make(map[UpstreamID]struct{})
		for _, svc := range resp.Services {
			uid := NewUpstreamIDFromServiceName(svc)
			seenUpstreams[uid] = struct{}{}
			{
				childCtx, cancel := context.WithCancel(ctx)
				err := s.dataSources.ConfigEntry.Notify(childCtx, &structs.ConfigEntryQuery{
					Kind:           structs.ServiceDefaults,
					Name:           svc.Name,
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					EnterpriseMeta: svc.EnterpriseMeta,
				}, DestinationConfigEntryID+svc.String(), s.ch)
				if err != nil {
					cancel()
					return err
				}
				snap.ConnectProxy.DestinationsUpstream.InitWatch(uid, cancel)
			}
			{
				childCtx, cancel := context.WithCancel(ctx)
				err := s.dataSources.ServiceGateways.Notify(childCtx, &structs.ServiceSpecificRequest{
					ServiceName:    svc.Name,
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					EnterpriseMeta: svc.EnterpriseMeta,
					ServiceKind:    structs.ServiceKindTerminatingGateway,
				}, DestinationGatewayID+svc.String(), s.ch)
				if err != nil {
					cancel()
					return err
				}
				snap.ConnectProxy.DestinationGateways.InitWatch(uid, cancel)
			}
		}

		snap.ConnectProxy.DestinationsUpstream.ForEachKey(func(uid UpstreamID) bool {
			if _, ok := seenUpstreams[uid]; !ok {
				snap.ConnectProxy.DestinationsUpstream.CancelWatch(uid)
			}
			return true
		})

		snap.ConnectProxy.DestinationGateways.ForEachKey(func(uid UpstreamID) bool {
			if _, ok := seenUpstreams[uid]; !ok {
				snap.ConnectProxy.DestinationGateways.CancelWatch(uid)
			}
			return true
		})
	case strings.HasPrefix(u.CorrelationID, DestinationConfigEntryID):
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		pq := strings.TrimPrefix(u.CorrelationID, DestinationConfigEntryID)
		uid := UpstreamIDFromString(pq)
		serviceConf, ok := resp.Entry.(*structs.ServiceConfigEntry)
		if !ok {
			return fmt.Errorf("invalid type for service default: %T", resp.Entry.GetName())
		}

		snap.ConnectProxy.DestinationsUpstream.Set(uid, serviceConf)
	case strings.HasPrefix(u.CorrelationID, DestinationGatewayID):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		pq := strings.TrimPrefix(u.CorrelationID, DestinationGatewayID)
		uid := UpstreamIDFromString(pq)
		snap.ConnectProxy.DestinationGateways.Set(uid, resp.Nodes)
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

	default:
		return (*handlerUpstreams)(s).handleUpdateUpstreams(ctx, u, snap)
	}
	return nil
}
