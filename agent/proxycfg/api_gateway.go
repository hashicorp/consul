package proxycfg

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// handlerAPIGateway generates a new ConfigSnapshot in response to
// changes related to an api-gateway.
type handlerAPIGateway struct {
	handlerState
}

// initialize sets up the initial watches needed based on the api-gateway registration
func (h *handlerAPIGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
	snap := newConfigSnapshotFromServiceInstance(h.serviceInstance, h.stateConfig)

	// Watch for root changes
	err := h.dataSources.CARoots.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:   h.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: h.token},
		Source:       *h.source,
	}, rootsWatchID, h.ch)
	if err != nil {
		return snap, err
	}

	// Get information about the entire service mesh.
	err = h.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.MeshConfig,
		Name:           structs.MeshConfigMesh,
		Datacenter:     h.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: h.token},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(h.proxyID.PartitionOrDefault()),
	}, meshConfigEntryID, h.ch)
	if err != nil {
		return snap, err
	}

	// Watch the api-gateway's config entry
	err = h.subscribeToConfigEntry(ctx, structs.APIGateway, h.service, gatewayConfigWatchID)
	if err != nil {
		return snap, err
	}

	// Watch the bound-api-gateway's config entry
	err = h.subscribeToConfigEntry(ctx, structs.BoundAPIGateway, h.service, gatewayConfigWatchID)
	if err != nil {
		return snap, err
	}

	// Watch the api-gateway's list of upstreams
	err = h.dataSources.GatewayServices.Notify(ctx, &structs.ServiceSpecificRequest{
		Datacenter:     h.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: h.token},
		ServiceName:    h.service,
		EnterpriseMeta: h.proxyID.EnterpriseMeta,
	}, gatewayServicesWatchID, h.ch)
	if err != nil {
		return snap, err
	}

	snap.APIGateway.WatchedDiscoveryChains = make(map[UpstreamID]context.CancelFunc)
	snap.APIGateway.DiscoveryChain = make(map[UpstreamID]*structs.CompiledDiscoveryChain)
	snap.APIGateway.WatchedUpstreams = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.APIGateway.WatchedUpstreamEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.APIGateway.WatchedGateways = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.APIGateway.WatchedGatewayEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.APIGateway.Listeners = make(map[APIGatewayListenerKey]structs.APIGatewayListener)
	snap.APIGateway.UpstreamPeerTrustBundles = watch.NewMap[string, *pbpeering.PeeringTrustBundle]()
	snap.APIGateway.PeerUpstreamEndpoints = watch.NewMap[UpstreamID, structs.CheckServiceNodes]()
	snap.APIGateway.PeerUpstreamEndpointsUseHostnames = make(map[UpstreamID]struct{})
	return snap, nil
}

func (h *handlerAPIGateway) subscribeToConfigEntry(ctx context.Context, kind, name, watchID string) error {
	return h.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           kind,
		Name:           name,
		Datacenter:     h.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: h.token},
		EnterpriseMeta: h.proxyID.EnterpriseMeta,
	}, watchID, h.ch)
}

// handleUpdate responds to changes in the api-gateway
func (h *handlerAPIGateway) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	switch {
	case u.CorrelationID == rootsWatchID:
		// Handle change in the CA roots
		if err := h.handleRootCAUpdate(u, snap); err != nil {
			return err
		}
	case u.CorrelationID == gatewayConfigWatchID:
		// Handle change in the api-gateway or bound-api-gateway config entry
		if err := h.handleGatewayConfigUpdate(ctx, u, snap); err != nil {
			return err
		}
	case u.CorrelationID == inlineCertificateConfigWatchID:
		// Handle change in an attached inline-certificate config entry
		if err := h.handleInlineCertConfigUpdate(ctx, u, snap); err != nil {
			return err
		}
	case u.CorrelationID == routeConfigWatchID:
		// Handle change in an attached http-route or tcp-route config entry
		if err := h.handleRouteConfigUpdate(ctx, u, snap); err != nil {
			return err
		}
	case u.CorrelationID == gatewayServicesWatchID:
		// Handle change in the upstream services for the bound-api-gateway
		if err := h.handleGatewayServicesUpdate(ctx, u, snap); err != nil {
			return err
		}
	default:
		return (*handlerUpstreams)(h).handleUpdateUpstreams(ctx, u, snap)
	}

	return nil
}

// handleRootCAUpdate responsed to changes in the watched root CA for a gateway
func (h *handlerAPIGateway) handleRootCAUpdate(u UpdateEvent, snap *ConfigSnapshot) error {
	roots, ok := u.Result.(*structs.IndexedCARoots)
	if !ok {
		return fmt.Errorf("invalid type for response: %T", u.Result)
	}
	snap.Roots = roots
	return nil
}

// handleGatewayConfigUpdate responds to changes in the watched config entry for a gateway
func (h *handlerAPIGateway) handleGatewayConfigUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	resp, ok := u.Result.(*structs.ConfigEntryResponse)
	if !ok {
		return fmt.Errorf("invalid type for response: %T", u.Result)
	} else if resp.Entry == nil {
		return nil
	}

	switch gwConf := resp.Entry.(type) {
	case *structs.BoundAPIGatewayConfigEntry:
		for _, listener := range gwConf.Listeners {
			// Subscribe to changes in each attached x-route config entry
			for _, ref := range listener.Routes {
				err := h.subscribeToConfigEntry(ctx, ref.Kind, ref.Name, routeConfigWatchID)
				if err != nil {
					// TODO May want to continue
					return err
				}
			}

			// Subscribe to changes in each attached inline-certificate config entry
			for _, ref := range listener.Certificates {
				err := h.subscribeToConfigEntry(ctx, ref.Kind, ref.Name, inlineCertificateConfigWatchID)
				if err != nil {
					// TODO May want to continue
					return err
				}
			}
		}

		// TODO Unsubscribe from any config entries that are no longer attached

		// TODO
		break
	case *structs.APIGatewayConfigEntry:
		snap.APIGateway.GatewayConfigLoaded = true
		break
	default:
		return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
	}

	return h.watchIngressLeafCert(ctx, snap)
}

func (h *handlerAPIGateway) handleInlineCertConfigUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	resp, ok := u.Result.(*structs.ConfigEntryResponse)
	if !ok {
		return fmt.Errorf("invalid type for response: %T", u.Result)
	} else if resp.Entry == nil {
		return nil
	}

	_, ok = resp.Entry.(*structs.InlineCertificateConfigEntry)
	if !ok {
		return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
	}

	// TODO
	return errors.New("implement me")
}

func (h *handlerAPIGateway) handleRouteConfigUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	resp, ok := u.Result.(*structs.ConfigEntryResponse)
	if !ok {
		return fmt.Errorf("invalid type for response: %T", u.Result)
	} else if resp.Entry == nil {
		return nil
	}

	switch resp.Entry.(type) {
	case *structs.HTTPRouteConfigEntry:
		// TODO
		break
	case *structs.TCPRouteConfigEntry:
		// TODO
		break
	default:
		return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
	}

	return errors.New("implement me")
}

// handleGatewayServicesUpdate responds to changes in the set of watched upstreams for a gateway
func (h *handlerAPIGateway) handleGatewayServicesUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	services, ok := u.Result.(*structs.IndexedGatewayServices)
	if !ok {
		return fmt.Errorf("invalid type for response: %T", u.Result)
	}

	// Update our upstreams and watches.
	var hosts []string
	watchedSvcs := make(map[UpstreamID]struct{})
	upstreams := make(map[IngressListenerKey]structs.Upstreams)
	for _, service := range services.Services {
		upstream := makeUpstream(service)
		uid := NewUpstreamID(&upstream)

		// TODO(peering): pipe destination_peer here
		watchOpts := discoveryChainWatchOpts{
			id:         uid,
			name:       upstream.DestinationName,
			namespace:  upstream.DestinationNamespace,
			partition:  upstream.DestinationPartition,
			datacenter: h.source.Datacenter,
		}

		up := &handlerUpstreams{handlerState: h.handlerState}
		if err := up.watchDiscoveryChain(ctx, snap, watchOpts); err != nil {
			return fmt.Errorf("failed to watch discovery chain for %h: %v", uid, err)
		}
		watchedSvcs[uid] = struct{}{}

		hosts = append(hosts, service.Hosts...)

		id := IngressListenerKeyFromGWService(*service)
		upstreams[id] = append(upstreams[id], upstream)
	}

	snap.APIGateway.Upstreams = upstreams
	snap.APIGateway.UpstreamsSet = watchedSvcs
	snap.APIGateway.Hosts = hosts
	snap.APIGateway.HostsSet = true

	for uid, cancelFn := range snap.APIGateway.WatchedDiscoveryChains {
		if _, ok := watchedSvcs[uid]; !ok {
			for targetID, cancelUpstreamFn := range snap.APIGateway.WatchedUpstreams[uid] {
				delete(snap.APIGateway.WatchedUpstreams[uid], targetID)
				delete(snap.APIGateway.WatchedUpstreamEndpoints[uid], targetID)
				cancelUpstreamFn()

				targetUID := NewUpstreamIDFromTargetID(targetID)
				if targetUID.Peer != "" {
					snap.APIGateway.PeerUpstreamEndpoints.CancelWatch(targetUID)
					snap.APIGateway.UpstreamPeerTrustBundles.CancelWatch(targetUID.Peer)
				}
			}

			cancelFn()
			delete(snap.APIGateway.WatchedDiscoveryChains, uid)
		}
	}

	return h.watchIngressLeafCert(ctx, snap)
}

func (h *handlerAPIGateway) watchIngressLeafCert(ctx context.Context, snap *ConfigSnapshot) error {
	return errors.New("implement me")
}
