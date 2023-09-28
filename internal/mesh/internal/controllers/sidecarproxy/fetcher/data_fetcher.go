// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
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
	client pbresource.ResourceServiceClient
	cache  *cache.Cache
}

func New(client pbresource.ResourceServiceClient, cache *cache.Cache) *Fetcher {
	return &Fetcher{
		client: client,
		cache:  cache,
	}
}

func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*types.DecodedWorkload, error) {
	dec, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		// We also need to make sure to delete the associated proxy from cache.
		f.cache.UntrackWorkload(id)
		return nil, nil
	}

	f.cache.TrackWorkload(dec)

	return dec, err
}

func (f *Fetcher) FetchProxyStateTemplate(ctx context.Context, id *pbresource.ID) (*types.DecodedProxyStateTemplate, error) {
	return resource.GetDecodedResource[*pbmesh.ProxyStateTemplate](ctx, f.client, id)
}

func (f *Fetcher) FetchComputedTrafficPermissions(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedTrafficPermissions, error) {
	return resource.GetDecodedResource[*pbauth.ComputedTrafficPermissions](ctx, f.client, id)
}

func (f *Fetcher) FetchServiceEndpoints(ctx context.Context, id *pbresource.ID) (*types.DecodedServiceEndpoints, error) {
	return resource.GetDecodedResource[*pbcatalog.ServiceEndpoints](ctx, f.client, id)
}

func (f *Fetcher) FetchService(ctx context.Context, id *pbresource.ID) (*types.DecodedService, error) {
	return resource.GetDecodedResource[*pbcatalog.Service](ctx, f.client, id)
}

func (f *Fetcher) FetchDestinations(ctx context.Context, id *pbresource.ID) (*types.DecodedDestinations, error) {
	return resource.GetDecodedResource[*pbmesh.Destinations](ctx, f.client, id)
}

func (f *Fetcher) FetchComputedRoutes(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedRoutes, error) {
	if !types.IsComputedRoutesType(id.Type) {
		return nil, fmt.Errorf("id must be a ComputedRoutes type")
	}

	dec, err := resource.GetDecodedResource[*pbmesh.ComputedRoutes](ctx, f.client, id)
	if err != nil {
		return nil, err
	} else if dec == nil {
		f.cache.UntrackComputedRoutes(id)
	}

	return dec, err
}

func (f *Fetcher) FetchExplicitDestinationsData(
	ctx context.Context,
	proxyID *pbresource.ID,
) ([]*intermediateTypes.Destination, *intermediateTypes.Status, error) {

	var destinations []*intermediateTypes.Destination

	// Fetch computed explicit destinations first.
	cdID := resource.ReplaceType(pbmesh.ComputedExplicitDestinationsType, proxyID)
	cd, err := resource.GetDecodedResource[*pbmesh.ComputedExplicitDestinations](ctx, f.client, cdID)
	if err != nil {
		return nil, nil, err
	}
	if cd == nil {
		f.cache.UntrackComputedDestinations(cdID)
		return nil, nil, nil
	}

	// Otherwise, track this resource in the destinations cache.
	f.cache.TrackComputedDestinations(cd)

	status := &intermediateTypes.Status{
		ID:         cd.GetResource().GetId(),
		OldStatus:  cd.GetResource().GetStatus(),
		Generation: cd.GetResource().GetGeneration(),
	}

	for _, dest := range cd.GetData().GetDestinations() {
		d := &intermediateTypes.Destination{}

		var (
			serviceID  = resource.IDFromReference(dest.DestinationRef)
			serviceRef = resource.ReferenceToString(dest.DestinationRef)
		)

		// Fetch Service
		svc, err := f.FetchService(ctx, serviceID)
		if err != nil {
			return nil, status, err
		}

		if svc == nil {
			// If the Service resource is not found, then we update the status
			// of the ComputedDestinations resource.
			status.Conditions = append(status.Conditions, ctrlStatus.ConditionDestinationServiceNotFound(serviceRef))
			continue
		}

		d.Service = svc

		// Check if this service is mesh-enabled. If not, update the status.
		if !svc.GetData().IsMeshEnabled() {
			// Add invalid status.
			status.Conditions = append(status.Conditions, ctrlStatus.ConditionMeshProtocolNotFound(serviceRef))

			// This error should not cause the execution to stop, as we want to make sure that this non-mesh destination
			// service gets removed from the proxy state.
			continue
		}

		// Check if the desired port exists on the service and update the status if it doesn't.
		if svc.GetData().FindServicePort(dest.DestinationPort) == nil {
			status.Conditions = append(status.Conditions, ctrlStatus.ConditionDestinationPortNotFound(serviceRef, dest.DestinationPort))
			continue
		}

		// No destination port should point to a port with "mesh" protocol,
		// so check if destination port has the mesh protocol and update the status.
		if svc.GetData().FindServicePort(dest.DestinationPort).GetProtocol() == pbcatalog.Protocol_PROTOCOL_MESH {
			status.Conditions = append(status.Conditions, ctrlStatus.ConditionMeshProtocolDestinationPort(serviceRef, dest.DestinationPort))
			continue
		}

		// Fetch ComputedRoutes.
		cr, err := f.FetchComputedRoutes(ctx, resource.ReplaceType(pbmesh.ComputedRoutesType, serviceID))
		if err != nil {
			return nil, status, err
		} else if cr == nil {
			// This is required, so wait until it exists.
			status.Conditions = append(status.Conditions, ctrlStatus.ConditionDestinationComputedRoutesNotFound(serviceRef))
			continue
		}

		portConfig, ok := cr.Data.PortedConfigs[dest.DestinationPort]
		if !ok {
			// This is required, so wait until it exists.
			status.Conditions = append(status.Conditions,
				ctrlStatus.ConditionDestinationComputedRoutesPortNotFound(serviceRef, dest.DestinationPort))
			continue
		}

		// Copy this so we can mutate the targets.
		d.ComputedPortRoutes = proto.Clone(portConfig).(*pbmesh.ComputedPortRoutes)

		// As Destinations resource contains a list of destinations,
		// we need to find the one that references our service and port.
		d.Explicit = dest

		// NOTE: we collect both DIRECT and INDIRECT target information here.
		for _, routeTarget := range d.ComputedPortRoutes.Targets {
			targetServiceID := resource.IDFromReference(routeTarget.BackendRef.Ref)

			// Fetch ServiceEndpoints.
			se, err := f.FetchServiceEndpoints(ctx, resource.ReplaceType(pbcatalog.ServiceEndpointsType, targetServiceID))
			if err != nil {
				return nil, status, err
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

	// If we fetched and validated all destinations and ended up with no status conditions,
	// that means all destinations are valid, and we should set the status to a "happy" one.
	if len(status.Conditions) == 0 {
		status.Conditions = append(status.Conditions, ctrlStatus.ConditionAllDestinationsValid())
	}

	return destinations, status, nil
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
	rsp, err := f.client.List(ctx, &pbresource.ListRequest{
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

// FetchComputedProxyConfiguration fetches proxy configurations for the proxy state template provided by id
// and merges them into one object.
func (f *Fetcher) FetchComputedProxyConfiguration(ctx context.Context, id *pbresource.ID) (*types.DecodedComputedProxyConfiguration, error) {
	compProxyCfgID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, id)

	return resource.GetDecodedResource[*pbmesh.ComputedProxyConfiguration](ctx, f.client, compProxyCfgID)
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
