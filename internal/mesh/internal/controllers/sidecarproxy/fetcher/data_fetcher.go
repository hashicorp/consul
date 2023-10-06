// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	ctrlStatus "github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	intermediateTypes "github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Fetcher struct {
	Client              pbresource.ResourceServiceClient
	DestinationsCache   *sidecarproxycache.DestinationsCache
	ProxyCfgCache       *sidecarproxycache.ProxyConfigurationCache
	ComputedRoutesCache *sidecarproxycache.ComputedRoutesCache
	IdentitiesCache     *sidecarproxycache.IdentitiesCache
}

func New(
	client pbresource.ResourceServiceClient,
	dCache *sidecarproxycache.DestinationsCache,
	pcfgCache *sidecarproxycache.ProxyConfigurationCache,
	computedRoutesCache *sidecarproxycache.ComputedRoutesCache,
	iCache *sidecarproxycache.IdentitiesCache,
) *Fetcher {
	return &Fetcher{
		Client:              client,
		DestinationsCache:   dCache,
		ProxyCfgCache:       pcfgCache,
		ComputedRoutesCache: computedRoutesCache,
		IdentitiesCache:     iCache,
	}
}

func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*types.DecodedWorkload, error) {
	proxyID := resource.ReplaceType(pbmesh.ProxyStateTemplateType, id)
	dec, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, f.Client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// We also need to make sure to delete the associated proxy from cache.
		// We are ignoring errors from cache here as this deletion is best effort.
		f.DestinationsCache.DeleteSourceProxy(proxyID)
		f.ProxyCfgCache.UntrackProxyID(proxyID)
		f.IdentitiesCache.UntrackProxyID(proxyID)
		return nil, nil
	}

	identityID := &pbresource.ID{
		Name:    dec.Data.Identity,
		Tenancy: dec.Resource.Id.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}

	f.IdentitiesCache.TrackPair(identityID, proxyID)

	return dec, err
}

func (f *Fetcher) FetchProxyStateTemplate(ctx context.Context, id *pbresource.ID) (*types.DecodedProxyStateTemplate, error) {
	return resource.GetDecodedResource[*pbmesh.ProxyStateTemplate](ctx, f.Client, id)
}

func (f *Fetcher) FetchComputedTrafficPermissions(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedTrafficPermissions, error) {
	return resource.GetDecodedResource[*pbauth.ComputedTrafficPermissions](ctx, f.Client, id)
}

func (f *Fetcher) FetchServiceEndpoints(ctx context.Context, id *pbresource.ID) (*types.DecodedServiceEndpoints, error) {
	return resource.GetDecodedResource[*pbcatalog.ServiceEndpoints](ctx, f.Client, id)
}

func (f *Fetcher) FetchService(ctx context.Context, id *pbresource.ID) (*types.DecodedService, error) {
	return resource.GetDecodedResource[*pbcatalog.Service](ctx, f.Client, id)
}

func (f *Fetcher) FetchDestinations(ctx context.Context, id *pbresource.ID) (*types.DecodedDestinations, error) {
	return resource.GetDecodedResource[*pbmesh.Destinations](ctx, f.Client, id)
}

func (f *Fetcher) FetchComputedRoutes(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedRoutes, error) {
	if !types.IsComputedRoutesType(id.Type) {
		return nil, fmt.Errorf("id must be a ComputedRoutes type")
	}

	dec, err := resource.GetDecodedResource[*pbmesh.ComputedRoutes](ctx, f.Client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		f.ComputedRoutesCache.UntrackComputedRoutes(id)
	}

	return dec, err
}

func (f *Fetcher) FetchExplicitDestinationsData(
	ctx context.Context,
	explDestRefs []intermediateTypes.CombinedDestinationRef,
) ([]*intermediateTypes.Destination, map[string]*intermediateTypes.Status, error) {
	var (
		destinations []*intermediateTypes.Destination
		statuses     = make(map[string]*intermediateTypes.Status)
	)

	for _, dest := range explDestRefs {
		// Fetch Destinations resource if there is one.
		us, err := f.FetchDestinations(ctx, dest.ExplicitDestinationsID)
		if err != nil {
			// If there's an error, return and force another reconcile instead of computing
			// partial proxy state.
			return nil, statuses, err
		}

		if us == nil {
			// If the Destinations resource is not found, then we should delete it from cache and continue.
			f.DestinationsCache.DeleteDestination(dest.ServiceRef, dest.Port)
			continue
		}

		d := &intermediateTypes.Destination{}

		var (
			serviceID    = resource.IDFromReference(dest.ServiceRef)
			serviceRef   = resource.ReferenceToString(dest.ServiceRef)
			upstreamsRef = resource.IDToString(us.Resource.Id)
		)

		// Fetch Service
		svc, err := f.FetchService(ctx, serviceID)
		if err != nil {
			return nil, statuses, err
		}

		if svc == nil {
			// If the Service resource is not found, then we update the status
			// of the Upstreams resource but don't remove it from cache in case
			// it comes back.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionDestinationServiceNotFound(serviceRef))
			continue
		} else {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionDestinationServiceFound(serviceRef))
		}

		d.Service = svc

		// Check if this endpoints is mesh-enabled. If not, remove it from cache and return an error.
		if !IsMeshEnabled(svc.Data.Ports) {
			// Add invalid status but don't remove from cache. If this state changes,
			// we want to be able to detect this change.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionMeshProtocolNotFound(serviceRef))

			// This error should not cause the execution to stop, as we want to make sure that this non-mesh destination
			// gets removed from the proxy state.
			continue
		} else {
			// If everything was successful, add an empty condition so that we can remove any existing statuses.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionMeshProtocolFound(serviceRef))
		}

		// No destination port should point to a port with "mesh" protocol,
		// so check if destination port has the mesh protocol and update the status.
		if isServicePortMeshProtocol(svc.Data.Ports, dest.Port) {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionMeshProtocolDestinationPort(serviceRef, dest.Port))
			continue
		} else {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, dest.Port))
		}

		// Fetch ComputedRoutes.
		cr, err := f.FetchComputedRoutes(ctx, resource.ReplaceType(pbmesh.ComputedRoutesType, serviceID))
		if err != nil {
			return nil, statuses, err
		} else if cr == nil {
			// This is required, so wait until it exists.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation,
				ctrlStatus.ConditionDestinationComputedRoutesNotFound(serviceRef))
			continue
		} else {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation,
				ctrlStatus.ConditionDestinationComputedRoutesFound(serviceRef))
		}

		portConfig, ok := cr.Data.PortedConfigs[dest.Port]
		if !ok {
			// This is required, so wait until it exists.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation,
				ctrlStatus.ConditionDestinationComputedRoutesPortNotFound(serviceRef, dest.Port))
			continue
		} else {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation,
				ctrlStatus.ConditionDestinationComputedRoutesPortFound(serviceRef, dest.Port))
		}
		// Copy this so we can mutate the targets.
		d.ComputedPortRoutes = proto.Clone(portConfig).(*pbmesh.ComputedPortRoutes)

		// As Destinations resource contains a list of destinations,
		// we need to find the one that references our service and port.
		d.Explicit = findDestination(dest.ServiceRef, dest.Port, us.Data)
		if d.Explicit == nil {
			continue // the cache is out of sync
		}

		// NOTE: we collect both DIRECT and INDIRECT target information here.
		for _, routeTarget := range d.ComputedPortRoutes.Targets {
			targetServiceID := resource.IDFromReference(routeTarget.BackendRef.Ref)

			// Fetch ServiceEndpoints.
			se, err := f.FetchServiceEndpoints(ctx, resource.ReplaceType(pbcatalog.ServiceEndpointsType, targetServiceID))
			if err != nil {
				return nil, statuses, err
			}

			if se != nil {
				routeTarget.ServiceEndpointsId = se.Resource.Id
				routeTarget.ServiceEndpoints = se.Data

				// Gather all identities.
				var identities []*pbresource.Reference
				for _, ep := range se.Data.Endpoints {
					identities = append(identities, &pbresource.Reference{
						Name:    ep.Identity,
						Tenancy: se.Resource.Id.Tenancy,
					})
				}
				routeTarget.IdentityRefs = identities
			}
		}

		destinations = append(destinations, d)
	}

	return destinations, statuses, nil
}

type PortReferenceKey struct {
	resource.ReferenceKey
	Port string
}

// FetchImplicitDestinationsData fetches all implicit destinations and adds them to existing destinations.
// If the implicit destination is already in addToDestinations, it will be skipped.
// todo (ishustava): this function will eventually need to fetch implicit destinations from the ImplicitDestinations resource instead.
func (f *Fetcher) FetchImplicitDestinationsData(
	ctx context.Context,
	proxyID *pbresource.ID,
	addToDestinations []*intermediateTypes.Destination,
) ([]*intermediateTypes.Destination, error) {
	// First, convert existing destinations to a map so we can de-dup.
	//
	// This is keyed by the serviceID+port of the upstream, which is effectively
	// the same as the id of the computed routes for the service.
	destinations := make(map[PortReferenceKey]*intermediateTypes.Destination)
	for _, d := range addToDestinations {
		prk := PortReferenceKey{
			ReferenceKey: resource.NewReferenceKey(d.Service.Resource.Id),
			Port:         d.ComputedPortRoutes.ParentRef.Port,
		}
		destinations[prk] = d
	}

	// For now we need to look up all computed routes within a partition.
	rsp, err := f.Client.List(ctx, &pbresource.ListRequest{
		Type: pbmesh.ComputedRoutesType,
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: proxyID.Tenancy.Partition,
			PeerName:  proxyID.Tenancy.PeerName,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, r := range rsp.Resources {
		svcID := resource.ReplaceType(pbcatalog.ServiceType, r.Id)
		computedRoutes, err := resource.Decode[*pbmesh.ComputedRoutes](r)
		if err != nil {
			return nil, err
		}

		if computedRoutes == nil {
			// the routes-controller doesn't deem this worthy of the mesh
			continue
		}

		// Fetch the service.
		// todo (ishustava): this should eventually grab virtual IPs resource.
		svc, err := f.FetchService(ctx, resource.ReplaceType(pbcatalog.ServiceType, r.Id))
		if err != nil {
			return nil, err
		}
		if svc == nil {
			// If service no longer exists, skip.
			continue
		}

		// If this proxy is a part of this service, ignore it.
		if isPartOfService(resource.ReplaceType(pbcatalog.WorkloadType, proxyID), svc) {
			continue
		}

		inMesh := false
		for _, port := range svc.Data.Ports {
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				inMesh = true
				break
			}
		}

		if !inMesh {
			// If a service is no longer in the mesh, skip.
			continue
		}

		// Fetch the resources that may show up duplicated.
		//
		// NOTE: we collect both DIRECT and INDIRECT target information here.
		endpointsMap := make(map[resource.ReferenceKey]*types.DecodedServiceEndpoints)
		for _, portConfig := range computedRoutes.Data.PortedConfigs {
			for _, routeTarget := range portConfig.Targets {
				targetServiceID := resource.IDFromReference(routeTarget.BackendRef.Ref)

				seID := resource.ReplaceType(pbcatalog.ServiceEndpointsType, targetServiceID)
				seRK := resource.NewReferenceKey(seID)

				if _, ok := endpointsMap[seRK]; !ok {
					se, err := f.FetchServiceEndpoints(ctx, seID)
					if err != nil {
						return nil, err
					}
					// We only add the endpoint to the map if it's not nil. If it's missing on lookup now, the
					// controller should get triggered when the endpoint exists again since it watches service
					// endpoints.
					if se != nil {
						endpointsMap[seRK] = se
					}
				}
			}
		}

		for portName, portConfig := range computedRoutes.Data.PortedConfigs {
			// If it's already in destinations, ignore it.
			prk := PortReferenceKey{
				ReferenceKey: resource.NewReferenceKey(svcID),
				Port:         portName,
			}
			if _, ok := destinations[prk]; ok {
				continue
			}

			// Copy this so we can mutate the targets.
			portConfig = proto.Clone(portConfig).(*pbmesh.ComputedPortRoutes)

			d := &intermediateTypes.Destination{
				Service:            svc,
				ComputedPortRoutes: portConfig,
				VirtualIPs:         svc.Data.VirtualIps,
			}
			for _, routeTarget := range portConfig.Targets {
				targetServiceID := resource.IDFromReference(routeTarget.BackendRef.Ref)
				seID := resource.ReplaceType(pbcatalog.ServiceEndpointsType, targetServiceID)

				// Fetch ServiceEndpoints.
				se, ok := endpointsMap[resource.NewReferenceKey(seID)]
				if ok {
					routeTarget.ServiceEndpointsId = se.Resource.Id
					routeTarget.ServiceEndpoints = se.Data

					// Gather all identities.
					var identities []*pbresource.Reference
					for _, ep := range se.Data.Endpoints {
						identities = append(identities, &pbresource.Reference{
							Name:    ep.Identity,
							Tenancy: se.Resource.Id.Tenancy,
						})
					}
					routeTarget.IdentityRefs = identities
				}
			}
			addToDestinations = append(addToDestinations, d)
		}
	}
	return addToDestinations, err
}

// FetchAndMergeProxyConfigurations fetches proxy configurations for the proxy state template provided by id
// and merges them into one object.
func (f *Fetcher) FetchAndMergeProxyConfigurations(ctx context.Context, id *pbresource.ID) (*pbmesh.ProxyConfiguration, error) {
	proxyCfgRefs := f.ProxyCfgCache.ProxyConfigurationsByProxyID(id)

	result := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{},
	}
	for _, ref := range proxyCfgRefs {
		proxyCfgID := &pbresource.ID{
			Name:    ref.GetName(),
			Type:    ref.GetType(),
			Tenancy: ref.GetTenancy(),
		}
		rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{
			Id: proxyCfgID,
		})
		switch {
		case status.Code(err) == codes.NotFound:
			f.ProxyCfgCache.UntrackProxyConfiguration(proxyCfgID)
			return nil, nil
		case err != nil:
			return nil, err
		}

		var proxyCfg pbmesh.ProxyConfiguration
		err = rsp.Resource.Data.UnmarshalTo(&proxyCfg)
		if err != nil {
			return nil, err
		}

		// Note that we only care about dynamic config as bootstrap config
		// will not be updated dynamically by this controller.
		// todo (ishustava): do sorting etc.
		proto.Merge(result.DynamicConfig, proxyCfg.DynamicConfig)
	}

	// Default the outbound listener port. If we don't do the nil check here, then BuildDestinations will panic creating
	// the outbound listener.
	if result.DynamicConfig.TransparentProxy == nil {
		result.DynamicConfig.TransparentProxy = &pbmesh.TransparentProxy{OutboundListenerPort: 15001}
	}

	return result, nil
}

// IsWorkloadMeshEnabled returns true if the workload or service endpoints port
// contain a port with the "mesh" protocol.
func IsWorkloadMeshEnabled(ports map[string]*pbcatalog.WorkloadPort) bool {
	for _, port := range ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			return true
		}
	}
	return false
}

// IsMeshEnabled returns true if the service ports contain a port with the
// "mesh" protocol.
func IsMeshEnabled(ports []*pbcatalog.ServicePort) bool {
	for _, port := range ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			return true
		}
	}
	return false
}

func isServicePortMeshProtocol(ports []*pbcatalog.ServicePort, name string) bool {
	sp := findServicePort(ports, name)
	return sp != nil && sp.Protocol == pbcatalog.Protocol_PROTOCOL_MESH
}

func findServicePort(ports []*pbcatalog.ServicePort, name string) *pbcatalog.ServicePort {
	for _, port := range ports {
		if port.TargetPort == name {
			return port
		}
	}
	return nil
}

func findDestination(ref *pbresource.Reference, port string, destinations *pbmesh.Destinations) *pbmesh.Destination {
	for _, destination := range destinations.Destinations {
		if resource.EqualReference(ref, destination.DestinationRef) &&
			port == destination.DestinationPort {
			return destination
		}
	}
	return nil
}

func updateStatusCondition(
	statuses map[string]*intermediateTypes.Status,
	key string,
	id *pbresource.ID,
	oldStatus map[string]*pbresource.Status,
	generation string,
	condition *pbresource.Condition) {
	if _, ok := statuses[key]; ok {
		statuses[key].Conditions = append(statuses[key].Conditions, condition)
	} else {
		statuses[key] = &intermediateTypes.Status{
			ID:         id,
			Generation: generation,
			Conditions: []*pbresource.Condition{condition},
			OldStatus:  oldStatus,
		}
	}
}

func isPartOfService(workloadID *pbresource.ID, svc *types.DecodedService) bool {
	if !resource.EqualTenancy(workloadID.GetTenancy(), svc.Resource.Id.GetTenancy()) {
		return false
	}
	sel := svc.Data.Workloads
	for _, exact := range sel.GetNames() {
		if workloadID.GetName() == exact {
			return true
		}
	}
	for _, prefix := range sel.GetPrefixes() {
		if strings.HasPrefix(workloadID.GetName(), prefix) {
			return true
		}
	}
	return false
}
