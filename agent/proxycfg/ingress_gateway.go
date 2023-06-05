// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfg

import (
	"context"
	"fmt"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

type handlerIngressGateway struct {
	handlerState
}

func (s *handlerIngressGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
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

	// Watch this ingress gateway's config entry
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.IngressGateway,
		Name:           s.service,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, gatewayConfigWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch the ingress-gateway's list of upstreams
	err = s.dataSources.GatewayServices.Notify(ctx, &structs.ServiceSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		ServiceName:    s.service,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, gatewayServicesWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	snap.IngressGateway.WatchedDiscoveryChains = make(map[UpstreamID]context.CancelFunc)
	snap.IngressGateway.DiscoveryChain = make(map[UpstreamID]*structs.CompiledDiscoveryChain)
	snap.IngressGateway.WatchedUpstreams = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.IngressGateway.WatchedUpstreamEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.IngressGateway.WatchedGateways = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.IngressGateway.WatchedGatewayEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.IngressGateway.WatchedLocalGWEndpoints = watch.NewMap[string, structs.CheckServiceNodes]()
	snap.IngressGateway.Listeners = make(map[IngressListenerKey]structs.IngressListener)
	snap.IngressGateway.UpstreamPeerTrustBundles = watch.NewMap[string, *pbpeering.PeeringTrustBundle]()
	snap.IngressGateway.PeerUpstreamEndpoints = watch.NewMap[UpstreamID, structs.CheckServiceNodes]()
	snap.IngressGateway.PeerUpstreamEndpointsUseHostnames = make(map[UpstreamID]struct{})
	return snap, nil
}

func (s *handlerIngressGateway) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
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

		// We set this even if the response is empty so that we know the watch is set,
		// but we don't block if the ingress config entry is unset for this gateway
		snap.IngressGateway.GatewayConfigLoaded = true

		if resp.Entry == nil {
			return nil
		}
		gatewayConf, ok := resp.Entry.(*structs.IngressGatewayConfigEntry)
		if !ok {
			return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
		}

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

// Note: Ingress gateways are always bound to ports and never unix sockets.
// This means LocalBindPort is the only possibility
func makeUpstream(g *structs.GatewayService) structs.Upstream {
	upstream := structs.Upstream{
		DestinationName:      g.Service.Name,
		DestinationNamespace: g.Service.NamespaceOrDefault(),
		DestinationPartition: g.Service.PartitionOrDefault(),
		LocalBindPort:        g.Port,
		IngressHosts:         g.Hosts,
		// Pass the protocol that was configured on the ingress listener in order
		// to force that protocol on the Envoy listener.
		Config: map[string]interface{}{
			"protocol": g.Protocol,
		},
	}

	return upstream
}

func (s *handlerIngressGateway) watchIngressLeafCert(ctx context.Context, snap *ConfigSnapshot) error {
	// Note that we DON'T test for TLS.enabled because we need a leaf cert for the
	// gateway even without TLS to use as a client cert.
	if !snap.IngressGateway.GatewayConfigLoaded || !snap.IngressGateway.HostsSet {
		return nil
	}

	// Watch the leaf cert
	if snap.IngressGateway.LeafCertWatchCancel != nil {
		snap.IngressGateway.LeafCertWatchCancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	err := s.dataSources.LeafCertificate.Notify(ctx, &cachetype.ConnectCALeafRequest{
		Datacenter:     s.source.Datacenter,
		Token:          s.token,
		Service:        s.service,
		DNSSAN:         s.generateIngressDNSSANs(snap),
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, leafWatchID, s.ch)
	if err != nil {
		cancel()
		return err
	}
	snap.IngressGateway.LeafCertWatchCancel = cancel

	return nil
}

// connectTLSServingEnabled returns true if Connect TLS is enabled at either
// gateway level or for at least one of the specific listeners.
func connectTLSServingEnabled(snap *ConfigSnapshot) bool {
	if snap.IngressGateway.TLSConfig.Enabled {
		return true
	}

	for _, l := range snap.IngressGateway.Listeners {
		if l.TLS != nil && l.TLS.Enabled {
			return true
		}
	}
	return false
}

func (s *handlerIngressGateway) generateIngressDNSSANs(snap *ConfigSnapshot) []string {
	// Update our leaf cert watch with wildcard entries for our DNS domains as
	// well as any configured custom hostnames from the service. Note that in the
	// case that only a subset of listeners are TLS-enabled, we still load DNS
	// SANs for all upstreams. We could limit it to only those that are reachable
	// from the enabled listeners but that adds a lot of complication and they are
	// already wildcards anyway. It's simpler to have one certificate for the
	// whole proxy that works for any possible upstream we might need than try to
	// be more selective when we are already using wildcard DNS names!
	if !connectTLSServingEnabled(snap) {
		return nil
	}

	var dnsNames []string
	namespaces := make(map[string]struct{})
	for _, upstreams := range snap.IngressGateway.Upstreams {
		for _, u := range upstreams {
			namespaces[u.DestinationNamespace] = struct{}{}
		}
	}

	// TODO(partitions): How should these be updated for partitions?
	for ns := range namespaces {
		// The default namespace is special cased in DNS resolution, so special
		// case it here.
		if ns == structs.IntentionDefaultNamespace {
			ns = ""
		} else {
			ns = ns + "."
		}

		dnsNames = append(dnsNames, fmt.Sprintf("*.ingress.%s%s", ns, s.dnsConfig.Domain))
		dnsNames = append(dnsNames, fmt.Sprintf("*.ingress.%s%s.%s", ns, s.source.Datacenter, s.dnsConfig.Domain))
		if s.dnsConfig.AltDomain != "" {
			dnsNames = append(dnsNames, fmt.Sprintf("*.ingress.%s%s", ns, s.dnsConfig.AltDomain))
			dnsNames = append(dnsNames, fmt.Sprintf("*.ingress.%s%s.%s", ns, s.source.Datacenter, s.dnsConfig.AltDomain))
		}
	}

	dnsNames = append(dnsNames, snap.IngressGateway.Hosts...)

	return dnsNames
}
