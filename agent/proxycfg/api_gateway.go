package proxycfg

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// handlerAPIGateway generates a new ConfigSnapshot in response to
// changes related to an API gateway.
//
// It is currently identical to handlerIngressGateway but subscribes
// to the api-gateway config entry kind on initialize instead.
type handlerAPIGateway struct {
	handlerState
}

func (s *handlerAPIGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
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

	// Watch the api-gateway's config entry
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.APIGateway,
		Name:           s.service,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, gatewayConfigWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch the api-gateway's list of upstreams
	err = s.dataSources.GatewayServices.Notify(ctx, &structs.ServiceSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		ServiceName:    s.service,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, gatewayServicesWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	snap.APIGateway.WatchedDiscoveryChains = make(map[UpstreamID]context.CancelFunc)
	snap.APIGateway.DiscoveryChain = make(map[UpstreamID]*structs.CompiledDiscoveryChain)
	snap.APIGateway.WatchedUpstreams = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.APIGateway.WatchedUpstreamEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.APIGateway.WatchedGateways = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.APIGateway.WatchedGatewayEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.APIGateway.Listeners = make(map[IngressListenerKey]structs.IngressListener)
	snap.APIGateway.UpstreamPeerTrustBundles = watch.NewMap[string, *pbpeering.PeeringTrustBundle]()
	snap.APIGateway.PeerUpstreamEndpoints = watch.NewMap[UpstreamID, structs.CheckServiceNodes]()
	snap.APIGateway.PeerUpstreamEndpointsUseHostnames = make(map[UpstreamID]struct{})
	return snap, nil
}

func (s *handlerAPIGateway) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
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
	case u.CorrelationID == gatewayConfigWatchID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if resp.Entry == nil {
			return nil
		}
		gatewayConf, ok := resp.Entry.(*structs.IngressGatewayConfigEntry)
		if !ok {
			return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
		}

		snap.IngressGateway.GatewayConfigLoaded = true
		snap.IngressGateway.TLSConfig = gatewayConf.TLS
		if gatewayConf.Defaults != nil {
			snap.IngressGateway.Defaults = *gatewayConf.Defaults
		}

		// Load each listener's config from the config entry so we don't have to
		// pass listener config through "upstreams" types as that grows.
		for _, l := range gatewayConf.Listeners {
			key := IngressListenerKeyFromListener(l)
			snap.IngressGateway.Listeners[key] = l
		}

		if err := s.watchIngressLeafCert(ctx, snap); err != nil {
			return err
		}

	case u.CorrelationID == gatewayServicesWatchID:
		services, ok := u.Result.(*structs.IndexedGatewayServices)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		// Update our upstreams and watches.
		var hosts []string
		watchedSvcs := make(map[UpstreamID]struct{})
		upstreamsMap := make(map[IngressListenerKey]structs.Upstreams)
		for _, service := range services.Services {
			u := makeUpstream(service)

			uid := NewUpstreamID(&u)

			// TODO(peering): pipe destination_peer here
			watchOpts := discoveryChainWatchOpts{
				id:         uid,
				name:       u.DestinationName,
				namespace:  u.DestinationNamespace,
				partition:  u.DestinationPartition,
				datacenter: s.source.Datacenter,
			}
			up := &handlerUpstreams{handlerState: s.handlerState}
			err := up.watchDiscoveryChain(ctx, snap, watchOpts)
			if err != nil {
				return fmt.Errorf("failed to watch discovery chain for %s: %v", uid, err)
			}
			watchedSvcs[uid] = struct{}{}

			hosts = append(hosts, service.Hosts...)

			id := IngressListenerKeyFromGWService(*service)
			upstreamsMap[id] = append(upstreamsMap[id], u)
		}

		snap.IngressGateway.Upstreams = upstreamsMap
		snap.IngressGateway.UpstreamsSet = watchedSvcs
		snap.IngressGateway.Hosts = hosts
		snap.IngressGateway.HostsSet = true

		for uid, cancelFn := range snap.IngressGateway.WatchedDiscoveryChains {
			if _, ok := watchedSvcs[uid]; !ok {
				for targetID, cancelUpstreamFn := range snap.IngressGateway.WatchedUpstreams[uid] {
					delete(snap.IngressGateway.WatchedUpstreams[uid], targetID)
					delete(snap.IngressGateway.WatchedUpstreamEndpoints[uid], targetID)
					cancelUpstreamFn()

					targetUID := NewUpstreamIDFromTargetID(targetID)
					if targetUID.Peer != "" {
						snap.IngressGateway.PeerUpstreamEndpoints.CancelWatch(targetUID)
						snap.IngressGateway.UpstreamPeerTrustBundles.CancelWatch(targetUID.Peer)
					}
				}

				cancelFn()
				delete(snap.IngressGateway.WatchedDiscoveryChains, uid)
			}
		}

		if err := s.watchIngressLeafCert(ctx, snap); err != nil {
			return err
		}

	default:
		return (*handlerUpstreams)(s).handleUpdateUpstreams(ctx, u, snap)
	}

	return nil
}
