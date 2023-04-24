package proxycfg

import (
	"context"
	"fmt"
	"strings"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
)

type handlerTerminatingGateway struct {
	handlerState
}

// initialize sets up the initial watches needed based on the terminating-gateway registration
func (s *handlerTerminatingGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
	snap := newConfigSnapshotFromServiceInstance(s.serviceInstance, s.stateConfig)
	// Watch for root changes
	err := s.dataSources.CARoots.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		s.logger.Error("failed to register watch for root changes", "error", err)
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

	// Watch for the terminating-gateway's linked services
	err = s.dataSources.GatewayServices.Notify(ctx, &structs.ServiceSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		ServiceName:    s.service,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, gatewayServicesWatchID, s.ch)
	if err != nil {
		s.logger.Error("failed to register watch for linked services", "error", err)
		return snap, err
	}

	snap.TerminatingGateway.WatchedServices = make(map[structs.ServiceName]context.CancelFunc)
	snap.TerminatingGateway.WatchedIntentions = make(map[structs.ServiceName]context.CancelFunc)
	snap.TerminatingGateway.Intentions = make(map[structs.ServiceName]structs.Intentions)
	snap.TerminatingGateway.WatchedLeaves = make(map[structs.ServiceName]context.CancelFunc)
	snap.TerminatingGateway.ServiceLeaves = make(map[structs.ServiceName]*structs.IssuedCert)
	snap.TerminatingGateway.WatchedConfigs = make(map[structs.ServiceName]context.CancelFunc)
	snap.TerminatingGateway.ServiceConfigs = make(map[structs.ServiceName]*structs.ServiceConfigResponse)
	snap.TerminatingGateway.WatchedResolvers = make(map[structs.ServiceName]context.CancelFunc)
	snap.TerminatingGateway.ServiceResolvers = make(map[structs.ServiceName]*structs.ServiceResolverConfigEntry)
	snap.TerminatingGateway.ServiceResolversSet = make(map[structs.ServiceName]bool)
	snap.TerminatingGateway.ServiceGroups = make(map[structs.ServiceName]structs.CheckServiceNodes)
	snap.TerminatingGateway.GatewayServices = make(map[structs.ServiceName]structs.GatewayService)
	snap.TerminatingGateway.DestinationServices = make(map[structs.ServiceName]structs.GatewayService)
	snap.TerminatingGateway.HostnameServices = make(map[structs.ServiceName]structs.CheckServiceNodes)
	return snap, nil
}

func (s *handlerTerminatingGateway) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}
	logger := s.logger

	switch {
	case u.CorrelationID == rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.Roots = roots

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
			snap.TerminatingGateway.MeshConfig = meshConf
		} else {
			snap.TerminatingGateway.MeshConfig = nil
		}
		snap.TerminatingGateway.MeshConfigSet = true

	// Update watches based on the current list of services associated with the terminating-gateway
	case u.CorrelationID == gatewayServicesWatchID:
		services, ok := u.Result.(*structs.IndexedGatewayServices)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		svcMap := make(map[structs.ServiceName]struct{})
		for _, svc := range services.Services {
			// Make sure to add every service to this map, we use it to cancel watches below.
			svcMap[svc.Service] = struct{}{}

			// Store the gateway <-> service mapping for TLS origination
			if svc.ServiceKind == structs.GatewayServiceKindDestination {
				snap.TerminatingGateway.DestinationServices[svc.Service] = *svc
			} else {
				snap.TerminatingGateway.GatewayServices[svc.Service] = *svc
			}

			// Watch the health endpoint to discover endpoints for the service
			if _, ok := snap.TerminatingGateway.WatchedServices[svc.Service]; !ok && !(svc.ServiceKind == structs.GatewayServiceKindDestination) {

				ctx, cancel := context.WithCancel(ctx)
				err := s.dataSources.Health.Notify(ctx, &structs.ServiceSpecificRequest{
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					ServiceName:    svc.Service.Name,
					EnterpriseMeta: svc.Service.EnterpriseMeta,

					// The gateway acts as the service's proxy, so we do NOT want to discover other proxies
					Connect: false,
				}, externalServiceIDPrefix+svc.Service.String(), s.ch)

				if err != nil {
					logger.Error("failed to register watch for external-service",
						"service", svc.Service.String(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.TerminatingGateway.WatchedServices[svc.Service] = cancel
			}

			// Watch intentions with this service as their destination
			// The gateway will enforce intentions for connections to the service
			if _, ok := snap.TerminatingGateway.WatchedIntentions[svc.Service]; !ok {
				ctx, cancel := context.WithCancel(ctx)
				err := s.dataSources.Intentions.Notify(ctx, &structs.ServiceSpecificRequest{
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					EnterpriseMeta: svc.Service.EnterpriseMeta,
					ServiceName:    svc.Service.Name,
				}, serviceIntentionsIDPrefix+svc.Service.String(), s.ch)

				if err != nil {
					logger.Error("failed to register watch for service-intentions",
						"service", svc.Service.String(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.TerminatingGateway.WatchedIntentions[svc.Service] = cancel
			}

			// Watch leaf certificate for the service
			// This cert is used to terminate mTLS connections on the service's behalf
			if _, ok := snap.TerminatingGateway.WatchedLeaves[svc.Service]; !ok {
				ctx, cancel := context.WithCancel(ctx)
				err := s.dataSources.LeafCertificate.Notify(ctx, &cachetype.ConnectCALeafRequest{
					Datacenter:     s.source.Datacenter,
					Token:          s.token,
					Service:        svc.Service.Name,
					EnterpriseMeta: svc.Service.EnterpriseMeta,
				}, serviceLeafIDPrefix+svc.Service.String(), s.ch)

				if err != nil {
					logger.Error("failed to register watch for a service-leaf",
						"service", svc.Service.String(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.TerminatingGateway.WatchedLeaves[svc.Service] = cancel
			}

			// Watch service configs for the service.
			// These are used to determine the protocol for the target service.
			if _, ok := snap.TerminatingGateway.WatchedConfigs[svc.Service]; !ok {
				ctx, cancel := context.WithCancel(ctx)
				err := s.dataSources.ResolvedServiceConfig.Notify(ctx, &structs.ServiceConfigRequest{
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					Name:           svc.Service.Name,
					EnterpriseMeta: svc.Service.EnterpriseMeta,
				}, serviceConfigIDPrefix+svc.Service.String(), s.ch)

				if err != nil {
					logger.Error("failed to register watch for a resolved service config",
						"service", svc.Service.String(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.TerminatingGateway.WatchedConfigs[svc.Service] = cancel
			}

			// Watch service resolvers for the service
			// These are used to create clusters and endpoints for the service subsets
			if _, ok := snap.TerminatingGateway.WatchedResolvers[svc.Service]; !ok && !(svc.ServiceKind == structs.GatewayServiceKindDestination) {

				ctx, cancel := context.WithCancel(ctx)
				err := s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					Kind:           structs.ServiceResolver,
					Name:           svc.Service.Name,
					EnterpriseMeta: svc.Service.EnterpriseMeta,
				}, serviceResolverIDPrefix+svc.Service.String(), s.ch)

				if err != nil {
					logger.Error("failed to register watch for a service-resolver",
						"service", svc.Service.String(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.TerminatingGateway.WatchedResolvers[svc.Service] = cancel
			}
		}

		// Delete gateway service mapping for services that were not in the update
		for sn := range snap.TerminatingGateway.GatewayServices {
			if _, ok := svcMap[sn]; !ok {
				delete(snap.TerminatingGateway.GatewayServices, sn)
			}
		}

		// Delete endpoint service mapping for services that were not in the update
		for sn := range snap.TerminatingGateway.DestinationServices {
			if _, ok := svcMap[sn]; !ok {
				delete(snap.TerminatingGateway.DestinationServices, sn)
			}
		}

		// Clean up services with hostname mapping for services that were not in the update
		for sn := range snap.TerminatingGateway.HostnameServices {
			if _, ok := svcMap[sn]; !ok {
				delete(snap.TerminatingGateway.HostnameServices, sn)
			}
		}

		// Cancel service instance watches for services that were not in the update
		for sn, cancelFn := range snap.TerminatingGateway.WatchedServices {
			if _, ok := svcMap[sn]; !ok {
				logger.Debug("canceling watch for service", "service", sn.String())
				delete(snap.TerminatingGateway.WatchedServices, sn)
				delete(snap.TerminatingGateway.ServiceGroups, sn)
				cancelFn()
			}
		}

		// Cancel leaf cert watches for services that were not in the update
		for sn, cancelFn := range snap.TerminatingGateway.WatchedLeaves {
			if _, ok := svcMap[sn]; !ok {
				logger.Debug("canceling watch for leaf cert", "service", sn.String())
				delete(snap.TerminatingGateway.WatchedLeaves, sn)
				delete(snap.TerminatingGateway.ServiceLeaves, sn)
				cancelFn()
			}
		}

		// Cancel service config watches for services that were not in the update
		for sn, cancelFn := range snap.TerminatingGateway.WatchedConfigs {
			if _, ok := svcMap[sn]; !ok {
				logger.Debug("canceling watch for resolved service config", "service", sn.String())
				delete(snap.TerminatingGateway.WatchedConfigs, sn)
				delete(snap.TerminatingGateway.ServiceConfigs, sn)
				cancelFn()
			}
		}

		// Cancel service-resolver watches for services that were not in the update
		for sn, cancelFn := range snap.TerminatingGateway.WatchedResolvers {
			if _, ok := svcMap[sn]; !ok {
				logger.Debug("canceling watch for service-resolver", "service", sn.String())
				delete(snap.TerminatingGateway.WatchedResolvers, sn)
				delete(snap.TerminatingGateway.ServiceResolvers, sn)
				delete(snap.TerminatingGateway.ServiceResolversSet, sn)
				cancelFn()
			}
		}

		// Cancel intention watches for services that were not in the update
		for sn, cancelFn := range snap.TerminatingGateway.WatchedIntentions {
			if _, ok := svcMap[sn]; !ok {
				logger.Debug("canceling watch for intention", "service", sn.String())
				delete(snap.TerminatingGateway.WatchedIntentions, sn)
				delete(snap.TerminatingGateway.Intentions, sn)
				cancelFn()
			}
		}

	case strings.HasPrefix(u.CorrelationID, externalServiceIDPrefix):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, externalServiceIDPrefix))
		delete(snap.TerminatingGateway.ServiceGroups, sn)
		delete(snap.TerminatingGateway.HostnameServices, sn)

		if len(resp.Nodes) > 0 {
			snap.TerminatingGateway.ServiceGroups[sn] = resp.Nodes
			snap.TerminatingGateway.HostnameServices[sn] = hostnameEndpoints(
				s.logger,
				snap.Locality,
				resp.Nodes,
			)
		}

	// Store leaf cert for watched service
	case strings.HasPrefix(u.CorrelationID, serviceLeafIDPrefix):
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, serviceLeafIDPrefix))
		snap.TerminatingGateway.ServiceLeaves[sn] = leaf

	case strings.HasPrefix(u.CorrelationID, serviceConfigIDPrefix):
		serviceConfig, ok := u.Result.(*structs.ServiceConfigResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, serviceConfigIDPrefix))
		snap.TerminatingGateway.ServiceConfigs[sn] = serviceConfig

	case strings.HasPrefix(u.CorrelationID, serviceResolverIDPrefix):
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, serviceResolverIDPrefix))
		// There should only ever be one entry for a service resolver within a namespace
		if resolver, ok := resp.Entry.(*structs.ServiceResolverConfigEntry); ok {
			snap.TerminatingGateway.ServiceResolvers[sn] = resolver
			snap.TerminatingGateway.ServiceResolversSet[sn] = true
		} else {
			// we likely have a deleted service resolver, and our cast is a nil
			// cast, so clear this out
			delete(snap.TerminatingGateway.ServiceResolvers, sn)
			snap.TerminatingGateway.ServiceResolversSet[sn] = false
		}

	case strings.HasPrefix(u.CorrelationID, serviceIntentionsIDPrefix):
		resp, ok := u.Result.(structs.Intentions)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, serviceIntentionsIDPrefix))
		snap.TerminatingGateway.Intentions[sn] = resp

	default:
		// do nothing
	}

	return nil
}
