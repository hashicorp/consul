package proxycfg

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/maps"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/pbpeering"
)

type handlerMeshGateway struct {
	handlerState
}

// initialize sets up the watches needed based on the current mesh gateway registration
func (s *handlerMeshGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
	snap := newConfigSnapshotFromServiceInstance(s.serviceInstance, s.stateConfig)
	// Watch for root changes
	err := s.dataSources.CARoots.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch for all peer trust bundles we may need.
	if s.peeringEnabled {
		err = s.dataSources.TrustBundleList.Notify(ctx, &cachetype.TrustBundleListRequest{
			Request: &pbpeering.TrustBundleListByServiceRequest{
				Kind:        string(structs.ServiceKindMeshGateway),
				ServiceName: s.service,
				Namespace:   s.proxyID.NamespaceOrDefault(),
				Partition:   s.proxyID.PartitionOrDefault(),
			},
			QueryOptions: structs.QueryOptions{Token: s.token},
		}, peeringTrustBundlesWatchID, s.ch)
		if err != nil {
			return snap, err
		}
	} else {
		// Initialize these fields even when peering is not enabled, so that the mesh gateway
		// returns the proper value in calls to `Valid()`
		snap.MeshGateway.PeeringTrustBundles = make([]*pbpeering.PeeringTrustBundle, 0)
		snap.MeshGateway.PeeringTrustBundlesSet = true
	}

	wildcardEntMeta := s.proxyID.WithWildcardNamespace()

	// Watch for all services.
	// Eventually we will have to watch connect enabled instances for each service as well as the
	// destination services themselves but those notifications will be setup later.
	// We cannot setup those watches until we know what the services are.
	err = s.dataSources.ServiceList.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		Source:         *s.source,
		EnterpriseMeta: *wildcardEntMeta,
	}, serviceListWatchID, s.ch)

	if err != nil {
		return snap, err
	}

	// Watch service-resolvers so we can setup service subset clusters
	err = s.dataSources.ConfigEntryList.Notify(ctx, &structs.ConfigEntryQuery{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		Kind:           structs.ServiceResolver,
		EnterpriseMeta: *wildcardEntMeta,
	}, serviceResolversWatchID, s.ch)
	if err != nil {
		s.logger.Named(logging.MeshGateway).
			Error("failed to register watch for service-resolver config entries", "error", err)
		return snap, err
	}

	if s.proxyID.InDefaultPartition() {
		if err := s.initializeCrossDCWatches(ctx); err != nil {
			return snap, err
		}
	}

	if err := s.initializeEntWatches(ctx); err != nil {
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

	// Watch for all exported services from this mesh gateway's partition in any peering.
	err = s.dataSources.ExportedPeeredServices.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		Source:         *s.source,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, exportedServiceListWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	snap.MeshGateway.WatchedServices = make(map[structs.ServiceName]context.CancelFunc)
	snap.MeshGateway.WatchedGateways = make(map[string]context.CancelFunc)
	snap.MeshGateway.ServiceGroups = make(map[structs.ServiceName]structs.CheckServiceNodes)
	snap.MeshGateway.GatewayGroups = make(map[string]structs.CheckServiceNodes)
	snap.MeshGateway.ServiceResolvers = make(map[structs.ServiceName]*structs.ServiceResolverConfigEntry)
	snap.MeshGateway.HostnameDatacenters = make(map[string]structs.CheckServiceNodes)
	snap.MeshGateway.ExportedServicesWithPeers = make(map[structs.ServiceName][]string)
	snap.MeshGateway.DiscoveryChain = make(map[structs.ServiceName]*structs.CompiledDiscoveryChain)
	snap.MeshGateway.WatchedDiscoveryChains = make(map[structs.ServiceName]context.CancelFunc)

	// there is no need to initialize the map of service resolvers as we
	// fully rebuild it every time we get updates
	return snap, err
}

func (s *handlerMeshGateway) initializeCrossDCWatches(ctx context.Context) error {
	if s.meta[structs.MetaWANFederationKey] == "1" {
		// Conveniently we can just use this service meta attribute in one
		// place here to set the machinery in motion and leave the conditional
		// behavior out of the rest of the package.
		err := s.dataSources.FederationStateListMeshGateways.Notify(ctx, &structs.DCSpecificRequest{
			Datacenter:   s.source.Datacenter,
			QueryOptions: structs.QueryOptions{Token: s.token},
			Source:       *s.source,
		}, federationStateListGatewaysWatchID, s.ch)
		if err != nil {
			return err
		}

		err = s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
			Datacenter:   s.source.Datacenter,
			QueryOptions: structs.QueryOptions{Token: s.token},
			ServiceName:  structs.ConsulServiceName,
		}, consulServerListWatchID, s.ch)
		if err != nil {
			return err
		}
	}

	err := s.dataSources.Datacenters.Notify(ctx, &structs.DatacentersRequest{
		QueryOptions: structs.QueryOptions{Token: s.token, MaxAge: 30 * time.Second},
	}, datacentersWatchID, s.ch)
	if err != nil {
		return err
	}

	// Once we start getting notified about the datacenters we will setup watches on the
	// gateways within those other datacenters. We cannot do that here because we don't
	// know what they are yet.

	return nil
}

func (s *handlerMeshGateway) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	meshLogger := s.logger.Named(logging.MeshGateway)

	switch u.CorrelationID {
	case rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.Roots = roots

	case federationStateListGatewaysWatchID:
		dcIndexedNodes, ok := u.Result.(*structs.DatacenterIndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.MeshGateway.FedStateGateways = dcIndexedNodes.DatacenterNodes

		for dc, nodes := range dcIndexedNodes.DatacenterNodes {
			snap.MeshGateway.HostnameDatacenters[dc] = hostnameEndpoints(
				s.logger.Named(logging.MeshGateway),
				snap.Locality,
				nodes,
			)
		}

		for dc := range snap.MeshGateway.HostnameDatacenters {
			if _, ok := dcIndexedNodes.DatacenterNodes[dc]; !ok {
				delete(snap.MeshGateway.HostnameDatacenters, dc)
			}
		}

	case serviceListWatchID:
		services, ok := u.Result.(*structs.IndexedServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		svcMap := make(map[structs.ServiceName]struct{})
		for _, svc := range services.Services {
			// Make sure to add every service to this map, we use it to cancel
			// watches below.
			svcMap[svc] = struct{}{}

			if _, ok := snap.MeshGateway.WatchedServices[svc]; !ok {
				ctx, cancel := context.WithCancel(ctx)
				err := s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					ServiceName:    svc.Name,
					Connect:        true,
					EnterpriseMeta: svc.EnterpriseMeta,
				}, fmt.Sprintf("connect-service:%s", svc.String()), s.ch)

				if err != nil {
					meshLogger.Error("failed to register watch for connect-service",
						"service", svc.String(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.MeshGateway.WatchedServices[svc] = cancel
			}
		}

		for sid, cancelFn := range snap.MeshGateway.WatchedServices {
			if _, ok := svcMap[sid]; !ok {
				meshLogger.Debug("canceling watch for service", "service", sid.String())
				// TODO (gateways) Should the sid also be deleted from snap.MeshGateway.ServiceGroups?
				//                 Do those endpoints get cleaned up some other way?
				delete(snap.MeshGateway.WatchedServices, sid)
				cancelFn()
			}
		}
		snap.MeshGateway.WatchedServicesSet = true

	case datacentersWatchID:
		datacentersRaw, ok := u.Result.(*[]string)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if datacentersRaw == nil {
			return fmt.Errorf("invalid response with a nil datacenter list")
		}

		datacenters := *datacentersRaw

		for _, dc := range datacenters {
			if dc == s.source.Datacenter {
				continue
			}

			entMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
			gk := GatewayKey{Datacenter: dc, Partition: entMeta.PartitionOrDefault()}

			if _, ok := snap.MeshGateway.WatchedGateways[gk.String()]; !ok {
				ctx, cancel := context.WithCancel(ctx)
				err := s.dataSources.InternalServiceDump.Notify(ctx, &structs.ServiceDumpRequest{
					Datacenter:     dc,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					ServiceKind:    structs.ServiceKindMeshGateway,
					UseServiceKind: true,
					Source:         *s.source,
					EnterpriseMeta: *entMeta,
				}, fmt.Sprintf("mesh-gateway:%s", gk.String()), s.ch)

				if err != nil {
					meshLogger.Error("failed to register watch for mesh-gateway",
						"datacenter", dc,
						"partition", entMeta.PartitionOrDefault(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.MeshGateway.WatchedGateways[gk.String()] = cancel
			}
		}

		for key, cancelFn := range snap.MeshGateway.WatchedGateways {
			gk := gatewayKeyFromString(key)
			if gk.Datacenter == s.source.Datacenter {
				// Only cross-DC watches are managed by the datacenters watch.
				continue
			}

			found := false
			for _, dcCurrent := range datacenters {
				if dcCurrent == gk.Datacenter {
					found = true
					break
				}
			}
			if !found {
				delete(snap.MeshGateway.WatchedGateways, key)
				cancelFn()
			}
		}

	case serviceResolversWatchID:
		configEntries, ok := u.Result.(*structs.IndexedConfigEntries)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		resolvers := make(map[structs.ServiceName]*structs.ServiceResolverConfigEntry)
		for _, entry := range configEntries.Entries {
			if resolver, ok := entry.(*structs.ServiceResolverConfigEntry); ok {
				resolvers[structs.NewServiceName(resolver.Name, &resolver.EnterpriseMeta)] = resolver
			}
		}
		snap.MeshGateway.ServiceResolvers = resolvers

	case consulServerListWatchID:
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		// Do some initial sanity checks to avoid doing something dumb.
		for _, csn := range resp.Nodes {
			if csn.Service.Service != structs.ConsulServiceName {
				return fmt.Errorf("expected service name %q but got %q",
					structs.ConsulServiceName, csn.Service.Service)
			}
			if csn.Node.Datacenter != snap.Datacenter {
				return fmt.Errorf("expected datacenter %q but got %q",
					snap.Datacenter, csn.Node.Datacenter)
			}
		}

		snap.MeshGateway.ConsulServers = resp.Nodes

	case exportedServiceListWatchID:
		exportedServices, ok := u.Result.(*structs.IndexedExportedServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		seenServices := make(map[structs.ServiceName][]string) // svc -> peername slice
		for peerName, services := range exportedServices.Services {
			for _, svc := range services {
				seenServices[svc] = append(seenServices[svc], peerName)
			}
		}
		// Sort the peer names so ultimately xDS has a stable output.
		for svc := range seenServices {
			sort.Strings(seenServices[svc])
		}
		peeredServiceList := maps.SliceOfKeys(seenServices)
		structs.ServiceList(peeredServiceList).Sort()

		snap.MeshGateway.ExportedServicesSlice = peeredServiceList
		snap.MeshGateway.ExportedServicesWithPeers = seenServices
		snap.MeshGateway.ExportedServicesSet = true

		// Decide if we do or do not need our leaf.
		hasExports := len(snap.MeshGateway.ExportedServicesSlice) > 0
		if hasExports && snap.MeshGateway.LeafCertWatchCancel == nil {
			// no watch and we need one
			ctx, cancel := context.WithCancel(ctx)
			err := s.dataSources.LeafCertificate.Notify(ctx, &cachetype.ConnectCALeafRequest{
				Datacenter:     s.source.Datacenter,
				Token:          s.token,
				Kind:           structs.ServiceKindMeshGateway,
				EnterpriseMeta: s.proxyID.EnterpriseMeta,
			}, leafWatchID, s.ch)
			if err != nil {
				cancel()
				return err
			}
			snap.MeshGateway.LeafCertWatchCancel = cancel
		} else if !hasExports && snap.MeshGateway.LeafCertWatchCancel != nil {
			// has watch and shouldn't
			snap.MeshGateway.LeafCertWatchCancel()
			snap.MeshGateway.LeafCertWatchCancel = nil
			snap.MeshGateway.Leaf = nil
		}

		// For each service that we should be exposing, also watch disco chains
		// in the same manner as an ingress gateway would.

		for _, svc := range snap.MeshGateway.ExportedServicesSlice {
			if _, ok := snap.MeshGateway.WatchedDiscoveryChains[svc]; ok {
				continue
			}

			ctx, cancel := context.WithCancel(ctx)
			err := s.dataSources.CompiledDiscoveryChain.Notify(ctx, &structs.DiscoveryChainRequest{
				Datacenter:           s.source.Datacenter,
				QueryOptions:         structs.QueryOptions{Token: s.token},
				Name:                 svc.Name,
				EvaluateInDatacenter: s.source.Datacenter,
				EvaluateInNamespace:  svc.NamespaceOrDefault(),
				EvaluateInPartition:  svc.PartitionOrDefault(),
			}, "discovery-chain:"+svc.String(), s.ch)
			if err != nil {
				meshLogger.Error("failed to register watch for discovery chain",
					"service", svc.String(),
					"error", err,
				)
				cancel()
				return err
			}

			snap.MeshGateway.WatchedDiscoveryChains[svc] = cancel
		}

		// Clean up data from services that were not in the update

		for svc, cancelFn := range snap.MeshGateway.WatchedDiscoveryChains {
			if _, ok := seenServices[svc]; !ok {
				cancelFn()
				delete(snap.MeshGateway.WatchedDiscoveryChains, svc)
			}
		}

		// These entries are intentionally handled separately from the
		// WatchedDiscoveryChains above. There have been situations where a
		// discovery watch was cancelled, then fired. That update event then
		// re-populated the DiscoveryChain map entry, which wouldn't get
		// cleaned up since there was no known watch for it.

		for svc := range snap.MeshGateway.DiscoveryChain {
			if _, ok := seenServices[svc]; !ok {
				delete(snap.MeshGateway.DiscoveryChain, svc)
			}
		}

	case leafWatchID:
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		if hasExports := len(snap.MeshGateway.ExportedServicesSlice) > 0; !hasExports {
			return nil // ignore this update, it's stale
		}

		snap.MeshGateway.Leaf = leaf

	case peeringTrustBundlesWatchID:
		if s.peeringEnabled {
			resp, ok := u.Result.(*pbpeering.TrustBundleListByServiceResponse)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}
			if len(resp.Bundles) > 0 {
				snap.MeshGateway.PeeringTrustBundles = resp.Bundles
			}
			snap.MeshGateway.PeeringTrustBundlesSet = true
		}

	case meshConfigEntryID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		if resp.Entry != nil {
			meshConf, ok := resp.Entry.(*structs.MeshConfigEntry)
			if !ok {
				return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
			}
			snap.MeshGateway.MeshConfig = meshConf
		} else {
			snap.MeshGateway.MeshConfig = nil
		}
		snap.MeshGateway.MeshConfigSet = true

	default:
		switch {
		case strings.HasPrefix(u.CorrelationID, "connect-service:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}

			sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, "connect-service:"))

			if len(resp.Nodes) > 0 {
				snap.MeshGateway.ServiceGroups[sn] = resp.Nodes
			} else if _, ok := snap.MeshGateway.ServiceGroups[sn]; ok {
				delete(snap.MeshGateway.ServiceGroups, sn)
			}

		case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
			resp, ok := u.Result.(*structs.IndexedNodesWithGateways)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}

			key := strings.TrimPrefix(u.CorrelationID, "mesh-gateway:")
			delete(snap.MeshGateway.GatewayGroups, key)
			delete(snap.MeshGateway.HostnameDatacenters, key)

			if len(resp.Nodes) > 0 {
				snap.MeshGateway.GatewayGroups[key] = resp.Nodes
				snap.MeshGateway.HostnameDatacenters[key] = hostnameEndpoints(
					s.logger.Named(logging.MeshGateway),
					snap.Locality,
					resp.Nodes,
				)
			}

		case strings.HasPrefix(u.CorrelationID, "discovery-chain:"):
			resp, ok := u.Result.(*structs.DiscoveryChainResponse)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}
			svcString := strings.TrimPrefix(u.CorrelationID, "discovery-chain:")
			svc := structs.ServiceNameFromString(svcString)

			if !snap.MeshGateway.IsServiceExported(svc) {
				delete(snap.MeshGateway.DiscoveryChain, svc)
				s.logger.Trace("discovery-chain watch fired for unknown service", "service", svc)
				return nil
			}

			snap.MeshGateway.DiscoveryChain[svc] = resp.Chain

		default:
			if err := s.handleEntUpdate(meshLogger, ctx, u, snap); err != nil {
				return err
			}
		}
	}

	return nil
}
