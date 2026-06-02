// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/maps"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

type handlerMeshGateway struct {
	handlerState
}

type peerAddressType string

const (
	undefinedAddressType peerAddressType = ""
	ipAddressType        peerAddressType = "ip"
	hostnameAddressType  peerAddressType = "hostname"
)

// initialize sets up the watches needed based on the current mesh gateway registration
func (s *handlerMeshGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
	snap := newConfigSnapshotFromServiceInstance(s.serviceInstance, s.stateConfig)
	snap.MeshGateway.WatchedLocalServers = watch.NewMap[string, structs.CheckServiceNodes]()

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
		if err := s.initializeCrossDCWatches(ctx, &snap); err != nil {
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
	// Watch for service default object that matches this mesh gateway's name
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.ServiceDefaults,
		Name:           s.service,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, serviceDefaultsWatchID, s.ch)
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
	snap.MeshGateway.WatchedPeeringServices = make(map[string]map[structs.ServiceName]context.CancelFunc)
	snap.MeshGateway.WatchedPeers = make(map[string]context.CancelFunc)
	snap.MeshGateway.PeeringServices = make(map[string]map[structs.ServiceName]PeeringServiceValue)
	snap.MeshGateway.ServicePorts = make(map[structs.ServiceName][]string)

	// there is no need to initialize the map of service resolvers as we
	// fully rebuild it every time we get updates
	return snap, err
}

func (s *handlerMeshGateway) initializeCrossDCWatches(ctx context.Context, snap *ConfigSnapshot) error {
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
		snap.MeshGateway.WatchedLocalServers.InitWatch(structs.ConsulServiceName, nil)
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

				// always remove the sid from the ServiceGroups when un-watch the service
				delete(snap.MeshGateway.ServiceGroups, sid)
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
					NodesOnly:      true,
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

		snap.MeshGateway.WatchedLocalServers.Set(structs.ConsulServiceName, resp.Nodes)

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

		if err := s.refreshMeshGatewayExportedServices(ctx, snap); err != nil {
			return err
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
		resp, ok := u.Result.(*pbpeering.TrustBundleListByServiceResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if len(resp.Bundles) > 0 {
			snap.MeshGateway.PeeringTrustBundles = resp.Bundles
		}
		snap.MeshGateway.PeeringTrustBundlesSet = true

		wildcardEntMeta := s.proxyID.WithWildcardNamespace()

		// For each peer, fetch the imported services to support mesh gateway local
		// mode.
		for _, tb := range resp.Bundles {
			entMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

			if _, ok := snap.MeshGateway.WatchedPeers[tb.PeerName]; !ok {
				ctx, cancel := context.WithCancel(ctx)

				err := s.dataSources.ServiceList.Notify(ctx, &structs.DCSpecificRequest{
					PeerName:       tb.PeerName,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					Source:         *s.source,
					EnterpriseMeta: *wildcardEntMeta,
				}, peeringServiceListWatchID+tb.PeerName, s.ch)

				if err != nil {
					meshLogger.Error("failed to register watch for mesh-gateway",
						"peer", tb.PeerName,
						"partition", entMeta.PartitionOrDefault(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.MeshGateway.WatchedPeers[tb.PeerName] = cancel
			}
		}

		for peerName, cancelFn := range snap.MeshGateway.WatchedPeers {
			found := false
			for _, bundle := range resp.Bundles {
				if peerName == bundle.PeerName {
					found = true
					break
				}
			}
			if !found {
				delete(snap.MeshGateway.PeeringServices, peerName)
				delete(snap.MeshGateway.WatchedPeers, peerName)
				delete(snap.MeshGateway.WatchedPeeringServices, peerName)
				cancelFn()
			}
		}

	case meshConfigEntryID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		meshConf, ok := resp.Entry.(*structs.MeshConfigEntry)
		if resp.Entry != nil && !ok {
			return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
		}
		snap.MeshGateway.MeshConfig = meshConf
		snap.MeshGateway.MeshConfigSet = true

		// If we're peering through mesh gateways it means the config entry may be deleted
		// or the flag was disabled. Here we clean up related watches if they exist.
		if !meshConf.PeerThroughMeshGateways() {
			// We avoid canceling server watches when WAN federation is enabled since it
			// always requires a watch to the local servers.
			if s.meta[structs.MetaWANFederationKey] != "1" {
				// If the entry was deleted we cancel watches that may have existed because of
				// PeerThroughMeshGateways being set in the past.
				snap.MeshGateway.WatchedLocalServers.CancelWatch(structs.ConsulServiceName)
			}
			if snap.MeshGateway.PeerServersWatchCancel != nil {
				snap.MeshGateway.PeerServersWatchCancel()
				snap.MeshGateway.PeerServersWatchCancel = nil

				snap.MeshGateway.PeerServers = nil
			}

			return nil
		}

		// If PeerThroughMeshGateways is enabled, and we are in the default partition,
		// we need to start watching the list of peering connections in all partitions
		// to set up outbound routes for the control plane. Consul servers are in the default partition,
		// so only mesh gateways here have his responsibility.
		if snap.ProxyID.InDefaultPartition() &&
			snap.MeshGateway.PeerServersWatchCancel == nil {

			peeringListCtx, cancel := context.WithCancel(ctx)
			err := s.dataSources.PeeringList.Notify(peeringListCtx, &cachetype.PeeringListRequest{
				Request: &pbpeering.PeeringListRequest{
					Partition: acl.WildcardPartitionName,
				},
				QueryOptions: structs.QueryOptions{Token: s.token},
			}, peerServersWatchID, s.ch)
			if err != nil {
				meshLogger.Error("failed to register watch for peering list", "error", err)
				cancel()
				return err
			}

			snap.MeshGateway.PeerServersWatchCancel = cancel
		}

		// We avoid initializing Consul server watches when WAN federation is enabled since it
		// always requires server watches.
		if s.meta[structs.MetaWANFederationKey] == "1" {
			return nil
		}

		if snap.MeshGateway.WatchedLocalServers.IsWatched(structs.ConsulServiceName) {
			return nil
		}

		notifyCtx, cancel := context.WithCancel(ctx)
		err := s.dataSources.Health.Notify(notifyCtx, &structs.ServiceSpecificRequest{
			Datacenter:   s.source.Datacenter,
			QueryOptions: structs.QueryOptions{Token: s.token},
			ServiceName:  structs.ConsulServiceName,
		}, consulServerListWatchID, s.ch)
		if err != nil {
			cancel()
			return fmt.Errorf("failed to watch local consul servers: %w", err)
		}

		snap.MeshGateway.WatchedLocalServers.InitWatch(structs.ConsulServiceName, cancel)

	case peerServersWatchID:
		resp, ok := u.Result.(*pbpeering.PeeringListResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		peerServers := make(map[string]PeerServersValue)
		for _, peering := range resp.Peerings {
			// We only need to keep track of outbound establish connections for mesh gateway.
			// We could also check for the peering status, but this requires a response from the leader
			// which holds the peerstream information. We want to allow stale reads so there could be peerings in
			// a deleting or terminating state.
			if !peering.ShouldDial() {
				continue
			}

			if existing, ok := peerServers[peering.PeerServerName]; ok && existing.Index >= peering.ModifyIndex {
				// Multiple peerings can reference the same set of Consul servers, since there can be
				// multiple partitions in a datacenter. Rather than randomly overwriting, we attempt to
				// use the latest addresses by checking the Raft index associated with the peering.
				continue
			}

			hostnames, ips := peerHostnamesAndIPs(meshLogger, peering.Name, peering.GetAddressesToDial())
			if len(hostnames) > 0 {
				peerServers[peering.PeerServerName] = PeerServersValue{
					Addresses: hostnames,
					Index:     peering.ModifyIndex,
					UseCDS:    true,
				}
			} else if len(ips) > 0 {
				peerServers[peering.PeerServerName] = PeerServersValue{
					Addresses: ips,
					Index:     peering.ModifyIndex,
				}
			}
		}

		snap.MeshGateway.PeerServers = peerServers
	case serviceDefaultsWatchID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
		}

		if resp.Entry == nil {
			return nil
		}
		serviceDefaults, ok := resp.Entry.(*structs.ServiceConfigEntry)
		if !ok {
			return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
		}

		if serviceDefaults.UpstreamConfig != nil && serviceDefaults.UpstreamConfig.Defaults != nil {
			if serviceDefaults.UpstreamConfig.Defaults.Limits != nil {
				snap.MeshGateway.Limits = serviceDefaults.UpstreamConfig.Defaults.Limits
			}
		}

	default:
		switch {
		case strings.HasPrefix(u.CorrelationID, peeringServiceListWatchID):
			services, ok := u.Result.(*structs.IndexedServiceList)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}

			peerName := strings.TrimPrefix(u.CorrelationID, peeringServiceListWatchID)

			svcMap := make(map[structs.ServiceName]struct{})

			if _, ok := snap.MeshGateway.WatchedPeeringServices[peerName]; !ok {
				snap.MeshGateway.WatchedPeeringServices[peerName] = make(map[structs.ServiceName]context.CancelFunc)
			}

			for _, svc := range services.Services {
				// Make sure to add every service to this map, we use it to cancel
				// watches below.
				svcMap[svc] = struct{}{}

				if _, ok := snap.MeshGateway.WatchedPeeringServices[peerName][svc]; !ok {
					ctx, cancel := context.WithCancel(ctx)
					err := s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
						PeerName:       peerName,
						QueryOptions:   structs.QueryOptions{Token: s.token},
						ServiceName:    svc.Name,
						Connect:        true,
						EnterpriseMeta: svc.EnterpriseMeta,
					}, fmt.Sprintf("peering-connect-service:%s:%s", peerName, svc.String()), s.ch)

					if err != nil {
						meshLogger.Error("failed to register watch for connect-service",
							"service", svc.String(),
							"error", err,
						)
						cancel()
						return err
					}
					snap.MeshGateway.WatchedPeeringServices[peerName][svc] = cancel
				}
			}

			watchedServices := snap.MeshGateway.WatchedPeeringServices[peerName]
			for sn, cancelFn := range watchedServices {
				if _, ok := svcMap[sn]; !ok {
					meshLogger.Debug("canceling watch for service", "service", sn.String())
					delete(snap.MeshGateway.WatchedPeeringServices[peerName], sn)
					delete(snap.MeshGateway.PeeringServices[peerName], sn)
					cancelFn()
				}
			}

		case strings.HasPrefix(u.CorrelationID, "connect-service:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}

			sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, "connect-service:"))

			if len(resp.Nodes) > 0 {
				snap.MeshGateway.ServiceGroups[sn] = resp.Nodes
				// Extract port names from service metadata
				portNames := parseServicePorts(resp.Nodes)
				if len(portNames) > 0 {
					snap.MeshGateway.ServicePorts[sn] = portNames
				} else {
					// No ports metadata, clean up any existing port data
					delete(snap.MeshGateway.ServicePorts, sn)
				}
			} else {
				delete(snap.MeshGateway.ServiceGroups, sn)
				delete(snap.MeshGateway.ServicePorts, sn)
			}
		case strings.HasPrefix(u.CorrelationID, "peering-connect-service:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)

			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}

			key := strings.TrimPrefix(u.CorrelationID, "peering-connect-service:")
			peer, snString, ok := strings.Cut(key, ":")

			if ok {
				sn := structs.ServiceNameFromString(snString)

				if len(resp.Nodes) > 0 {
					if _, ok := snap.MeshGateway.PeeringServices[peer]; !ok {
						snap.MeshGateway.PeeringServices[peer] = make(map[structs.ServiceName]PeeringServiceValue)
					}
					// Extract port names from service metadata (same as connect-service handler)
					portNames := parseServicePorts(resp.Nodes)
					if len(portNames) > 0 {
						snap.MeshGateway.ServicePorts[sn] = portNames
					} else {
						// No ports metadata, clean up any existing port data
						delete(snap.MeshGateway.ServicePorts, sn)
					}

					if eps := hostnameEndpoints(s.logger, GatewayKey{}, resp.Nodes); len(eps) > 0 {
						snap.MeshGateway.PeeringServices[peer][sn] = PeeringServiceValue{
							Nodes:  eps,
							UseCDS: true,
						}
					} else {
						snap.MeshGateway.PeeringServices[peer][sn] = PeeringServiceValue{
							Nodes: resp.Nodes,
						}
					}
				} else if _, ok := snap.MeshGateway.PeeringServices[peer]; ok {
					delete(snap.MeshGateway.PeeringServices[peer], sn)
					delete(snap.MeshGateway.ServicePorts, sn)
				}
			}

		case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
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

func (s *handlerMeshGateway) refreshMeshGatewayExportedServices(ctx context.Context, snap *ConfigSnapshot) error {
	meshLogger := s.logger.Named(logging.MeshGateway)

	allServices := make(map[structs.ServiceName]struct{})
	for svc := range snap.MeshGateway.ExportedServicesWithPeers {
		allServices[svc] = struct{}{}
	}
	s.addEntExportedServices(allServices, snap)

	exportedServices := maps.SliceOfKeys(allServices)
	structs.ServiceList(exportedServices).Sort()

	snap.MeshGateway.ExportedServicesSlice = exportedServices
	snap.MeshGateway.ExportedServicesSet = true

	hasExports := len(exportedServices) > 0
	if hasExports && snap.MeshGateway.LeafCertWatchCancel == nil {
		notifyCtx, cancel := context.WithCancel(ctx)
		err := s.dataSources.LeafCertificate.Notify(notifyCtx, &leafcert.ConnectCALeafRequest{
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
		snap.MeshGateway.LeafCertWatchCancel()
		snap.MeshGateway.LeafCertWatchCancel = nil
		snap.MeshGateway.Leaf = nil
	}

	for _, svc := range exportedServices {
		if _, ok := snap.MeshGateway.WatchedDiscoveryChains[svc]; ok {
			continue
		}

		notifyCtx, cancel := context.WithCancel(ctx)
		err := s.dataSources.CompiledDiscoveryChain.Notify(notifyCtx, &structs.DiscoveryChainRequest{
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

	for svc, cancelFn := range snap.MeshGateway.WatchedDiscoveryChains {
		if _, ok := allServices[svc]; !ok {
			cancelFn()
			delete(snap.MeshGateway.WatchedDiscoveryChains, svc)
		}
	}

	for svc := range snap.MeshGateway.DiscoveryChain {
		if _, ok := allServices[svc]; !ok {
			delete(snap.MeshGateway.DiscoveryChain, svc)
		}
	}

	return nil
}

func peerHostnamesAndIPs(logger hclog.Logger, peerName string, addresses []string) ([]structs.ServiceAddress, []structs.ServiceAddress) {
	var (
		hostnames []structs.ServiceAddress
		ips       []structs.ServiceAddress
	)

	// Sort the input so that the output is also sorted.
	sort.Strings(addresses)

	for _, addr := range addresses {
		ip, rawPort, splitErr := net.SplitHostPort(addr)
		port, convErr := strconv.Atoi(rawPort)

		if splitErr != nil || convErr != nil {
			logger.Warn("unable to parse ip and port from peer server address. skipping address.",
				"peer", peerName, "address", addr)
		}
		if net.ParseIP(ip) != nil {
			ips = append(ips, structs.ServiceAddress{
				Address: ip,
				Port:    port,
			})
		} else {
			hostnames = append(hostnames, structs.ServiceAddress{
				Address: ip,
				Port:    port,
			})
		}
	}

	if len(hostnames) > 0 && len(ips) > 0 {
		logger.Warn("peer server address list contains mix of hostnames and IP addresses; only hostnames will be passed to Envoy",
			"peer", peerName)
	}
	return hostnames, ips
}

// parseServicePorts extracts named port information from service instances.
// It prefers the canonical Service.Ports field and falls back to the legacy
// service metadata "ports" format used by earlier multiport experiments.
func parseServicePorts(nodes structs.CheckServiceNodes) []string {
	var portNames []string

	for _, node := range nodes {
		if node.Service == nil {
			continue
		}

		if len(node.Service.Ports) > 0 {
			for _, port := range node.Service.Ports {
				if port.Name == "" {
					continue
				}
				portNames = append(portNames, port.Name)
			}
			if len(portNames) > 0 {
				break
			}
		}

		if node.Service.Meta == nil {
			continue
		}

		portsStr, ok := node.Service.Meta["ports"]
		if !ok || portsStr == "" {
			continue
		}

		// Parse format: "api:8080,admin:9090,metrics:9102"
		pairs := strings.Split(portsStr, ",")
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}

			parts := strings.Split(pair, ":")
			if len(parts) != 2 {
				continue
			}

			portName := strings.TrimSpace(parts[0])
			portNumStr := strings.TrimSpace(parts[1])
			if portName == "" || strings.ContainsAny(portName, ".:") {
				continue
			}

			portNum, err := strconv.Atoi(portNumStr)
			if err != nil || portNum <= 0 || portNum > 65535 {
				continue
			}

			portNames = append(portNames, portName)
		}

		if len(portNames) > 0 {
			break
		}
	}

	return portNames
}
