// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbpeering"
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
	snap.ConnectProxy.WatchedLocalGWEndpoints = watch.NewMap[string, structs.CheckServiceNodes]()
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

	// Watch the leaf cert
	err = s.dataSources.LeafCertificate.Notify(ctx, &leafcert.ConnectCALeafRequest{
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

	// Watch for JWT provider updates.
	// While we could optimize by only watching providers referenced by intentions,
	// this should be okay because we expect few JWT providers and infrequent JWT
	// provider updates.
	err = s.dataSources.ConfigEntryList.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.JWTProvider,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(s.proxyID.PartitionOrDefault()),
	}, jwtProviderID, s.ch)
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
		NodeName:       s.source.Node,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, svcChecksWatchIDPrefix+structs.ServiceIDString(s.proxyCfg.DestinationServiceID, &s.proxyID.EnterpriseMeta), s.ch)
	if err != nil {
		return snap, err
	}

	if err := s.maybeInitializeTelemetryCollectorWatches(ctx, snap); err != nil {
		return snap, fmt.Errorf("failed to initialize telemetry collector watches: %w", err)
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
		err = s.dataSources.PeeredUpstreams.Notify(ctx, &structs.PartitionSpecificRequest{
			QueryOptions:   structs.QueryOptions{Token: s.token},
			Datacenter:     s.source.Datacenter,
			EnterpriseMeta: s.proxyID.EnterpriseMeta,
		}, peeredUpstreamsID, s.ch)
		if err != nil {
			return snap, err
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
			err := s.dataSources.PreparedQuery.Notify(ctx, &structs.PreparedQueryExecuteRequest{
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
			if u.DestinationPeer != "" {
				err := s.setupWatchesForPeeredUpstream(ctx, snap.ConnectProxy, NewUpstreamID(&u), dc)
				if err != nil {
					return snap, fmt.Errorf("failed to setup watches for peered upstream %q: %w", uid.String(), err)
				}
				continue
			}

			err := s.dataSources.CompiledDiscoveryChain.Notify(ctx, &structs.DiscoveryChainRequest{
				Datacenter:             s.source.Datacenter,
				QueryOptions:           structs.QueryOptions{Token: s.token},
				Name:                   u.DestinationName,
				EvaluateInDatacenter:   dc,
				EvaluateInNamespace:    ns,
				EvaluateInPartition:    partition,
				OverrideMeshGateway:    u.MeshGateway,
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

func (s *handlerConnectProxy) setupWatchesForPeeredUpstream(
	ctx context.Context,
	snapConnectProxy configSnapshotConnectProxy,
	uid UpstreamID,
	dc string,
) error {
	s.logger.Trace("initializing watch of peered upstream", "upstream", uid)

	// NOTE: An upstream that points to a peer by definition will
	// only ever watch a single catalog query, so a map key of just
	// "UID" is sufficient to cover the peer data watches here.
	err := s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
		PeerName:   uid.Peer,
		Datacenter: dc,
		QueryOptions: structs.QueryOptions{
			Token: s.token,
		},
		ServiceName:    uid.Name,
		Connect:        true,
		Source:         *s.source,
		EnterpriseMeta: uid.EnterpriseMeta,
	}, upstreamPeerWatchIDPrefix+uid.String(), s.ch)
	if err != nil {
		return fmt.Errorf("failed to watch health for %s: %v", uid, err)
	}
	snapConnectProxy.PeerUpstreamEndpoints.InitWatch(uid, nil)

	// Check whether a watch for this peer exists to avoid duplicates.
	if ok := snapConnectProxy.UpstreamPeerTrustBundles.IsWatched(uid.Peer); !ok {
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

		snapConnectProxy.UpstreamPeerTrustBundles.InitWatch(uid.Peer, cancel)
	}

	// Always watch local GW endpoints for peer upstreams so that we don't have to worry about
	// the timing on whether the wildcard upstream config was fetched yet.
	up := &handlerUpstreams{handlerState: s.handlerState}
	up.setupWatchForLocalGWEndpoints(ctx, &snapConnectProxy.ConfigSnapshotUpstreams)
	return nil
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

	case u.CorrelationID == peeringTrustBundlesWatchID:
		resp, ok := u.Result.(*pbpeering.TrustBundleListByServiceResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if len(resp.Bundles) > 0 {
			snap.ConnectProxy.InboundPeerTrustBundles = resp.Bundles
		}
		snap.ConnectProxy.InboundPeerTrustBundlesSet = true

	case u.CorrelationID == intentionsWatchID:
		resp, ok := u.Result.(structs.SimplifiedIntentions)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.ConnectProxy.Intentions = resp
		snap.ConnectProxy.IntentionsSet = true

	case u.CorrelationID == jwtProviderID:
		resp, ok := u.Result.(*structs.IndexedConfigEntries)

		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		providers := make(map[string]*structs.JWTProviderConfigEntry, len(resp.Entries))
		for _, entry := range resp.Entries {
			jwtEntry, ok := entry.(*structs.JWTProviderConfigEntry)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", entry)
			}
			providers[jwtEntry.Name] = jwtEntry
		}

		snap.JWTProviders = providers
	case u.CorrelationID == peeredUpstreamsID:
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

			err := s.setupWatchesForPeeredUpstream(ctx, snap.ConnectProxy, uid, s.source.Datacenter)
			if err != nil {
				return fmt.Errorf("failed to setup watches for peered upstream %q: %w", uid.String(), err)
			}
		}
		snap.ConnectProxy.PeeredUpstreams = seenUpstreams

		//
		// Clean up data
		//

		peeredChainTargets := make(map[UpstreamID]struct{})
		for _, discoChain := range snap.ConnectProxy.DiscoveryChain {
			for _, target := range discoChain.Targets {
				if target.Peer == "" {
					continue
				}
				uid := NewUpstreamIDFromTargetID(target.ID)
				peeredChainTargets[uid] = struct{}{}
			}
		}

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
			// Peered upstream came from a discovery chain target
			if _, ok := peeredChainTargets[uid]; ok {
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
				wildcardUID := NewWildcardUID(&s.proxyID.EnterpriseMeta)
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
				meshGateway = u.MeshGateway
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
				for targetID, cancelFn := range targets {
					cancelFn()

					targetUID := NewUpstreamIDFromTargetID(targetID)
					if targetUID.Peer != "" {
						snap.ConnectProxy.PeerUpstreamEndpoints.CancelWatch(targetUID)
						snap.ConnectProxy.UpstreamPeerTrustBundles.CancelWatch(targetUID.Peer)
					}
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

// telemetryCollectorConfig represents the basic opaque config values for pushing telemetry to
// a consul telemetry collector.
type telemetryCollectorConfig struct {
	// TelemetryCollectorBindSocketDir is a string that configures the directory for a
	// unix socket where Envoy will forward metrics. These metrics get pushed to
	// the Consul Telemetry collector.
	TelemetryCollectorBindSocketDir string `mapstructure:"envoy_telemetry_collector_bind_socket_dir"`
}

func parseTelemetryCollectorConfig(m map[string]interface{}) (telemetryCollectorConfig, error) {
	var cfg telemetryCollectorConfig
	err := mapstructure.WeakDecode(m, &cfg)

	if err != nil {
		return cfg, fmt.Errorf("failed to decode: %w", err)
	}

	return cfg, nil
}

// maybeInitializeTelemetryCollectorWatches will initialize a synthetic upstream and discovery chain
// watch for the consul telemetry collector, if telemetry data collection is enabled on the proxy registration.
func (s *handlerConnectProxy) maybeInitializeTelemetryCollectorWatches(ctx context.Context, snap ConfigSnapshot) error {
	cfg, err := parseTelemetryCollectorConfig(s.proxyCfg.Config)
	if err != nil {
		s.logger.Error("failed to parse connect.proxy.config", "error", err)
	}

	if cfg.TelemetryCollectorBindSocketDir == "" {
		// telemetry collection is not enabled, return early.
		return nil
	}

	// The path includes the proxy ID so that when multiple proxies are on the same host
	// they each have a distinct path to send their telemetry data.
	id := s.proxyID.NamespaceOrDefault() + "_" + s.proxyID.ID

	// UNIX domain sockets paths have a max length of 108, so we take a hash of the compound ID
	// to limit the length of the socket path.
	h := sha1.New()
	h.Write([]byte(id))
	hash := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	path := path.Join(cfg.TelemetryCollectorBindSocketDir, hash+".sock")

	upstream := structs.Upstream{
		DestinationNamespace: acl.DefaultNamespaceName,
		DestinationPartition: s.proxyID.PartitionOrDefault(),
		DestinationName:      api.TelemetryCollectorName,
		LocalBindSocketPath:  path,
		Config: map[string]interface{}{
			"protocol": "grpc",
		},
	}
	uid := NewUpstreamID(&upstream)
	snap.ConnectProxy.UpstreamConfig[uid] = &upstream

	err = s.dataSources.CompiledDiscoveryChain.Notify(ctx, &structs.DiscoveryChainRequest{
		Datacenter:           s.source.Datacenter,
		QueryOptions:         structs.QueryOptions{Token: s.token},
		Name:                 upstream.DestinationName,
		EvaluateInDatacenter: s.source.Datacenter,
		EvaluateInNamespace:  uid.NamespaceOrDefault(),
		EvaluateInPartition:  uid.PartitionOrDefault(),
	}, "discovery-chain:"+uid.String(), s.ch)
	if err != nil {
		return fmt.Errorf("failed to watch discovery chain for %s: %v", uid.String(), err)
	}
	return nil
}
