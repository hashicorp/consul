// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"context"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/meshgateways"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	intermediateTypes "github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func FetchUnifiedDestinationsData(
	ctx context.Context,
	rt controller.Runtime,
	workload *types.DecodedWorkload,
	mgwMode pbmesh.MeshGatewayMode,
	transparentProxyEnabled bool,
) ([]*intermediateTypes.Destination, error) {
	// Get all destinationsData.
	destinationsData, err := fetchComputedExplicitDestinationsData(rt, workload, mgwMode)
	if err != nil {
		return nil, err
	}

	if transparentProxyEnabled {
		rt.Logger.Trace("transparent proxy is enabled; fetching implicit destinations")
		destinationsData, err = fetchComputedImplicitDestinationsData(rt, workload, mgwMode, destinationsData)
		if err != nil {
			return nil, err
		}
	}
	return destinationsData, nil
}

func fetchComputedExplicitDestinationsData(
	rt controller.Runtime,
	workload *types.DecodedWorkload,
	mgwMode pbmesh.MeshGatewayMode,
) ([]*intermediateTypes.Destination, error) {
	cedID := resource.ReplaceType(pbmesh.ComputedExplicitDestinationsType, workload.Id)

	var destinations []*intermediateTypes.Destination

	// Fetch computed explicit destinations first.
	cd, err := cache.GetDecoded[*pbmesh.ComputedExplicitDestinations](rt.Cache, pbmesh.ComputedExplicitDestinationsType, "id", cedID)
	if err != nil {
		return nil, err
	} else if cd == nil {
		return nil, nil
	}

	for _, dest := range cd.GetData().GetDestinations() {
		serviceID := resource.IDFromReference(dest.DestinationRef)

		outDests, err := fetchSingleDestinationData(rt, workload.Id, mgwMode, serviceID, nil, dest)
		if err != nil {
			return nil, err
		} else if len(outDests) == 0 {
			continue // skip
		}

		// For explicit dests, we are guaranteed only one result.
		destinations = append(destinations, outDests[0])
	}

	return destinations, nil
}

type PortReferenceKey struct {
	resource.ReferenceKey
	Port string
}

// fetchImplicitDestinationsData fetches all implicit destinations and adds them to existing destinations.
// If the implicit destination is already in addToDestinations, it will be skipped.
//
// Rename to include computed term
func fetchComputedImplicitDestinationsData(
	rt controller.Runtime,
	workload *types.DecodedWorkload,
	mgwMode pbmesh.MeshGatewayMode,
	addToDestinations []*intermediateTypes.Destination,
) ([]*intermediateTypes.Destination, error) {
	cidID := &pbresource.ID{
		Type:    pbmesh.ComputedImplicitDestinationsType,
		Name:    workload.Data.Identity,
		Tenancy: workload.Id.Tenancy,
	}

	// First, convert existing destinations to a map so we can de-dup.
	//
	// This is keyed by the serviceID+port of the upstream, which is effectively
	// the same as the id of the computed routes for the service.
	destsSeen := make(map[PortReferenceKey]struct{})
	for _, d := range addToDestinations {
		prk := PortReferenceKey{
			ReferenceKey: resource.NewReferenceKey(d.Service.Resource.Id),
			Port:         d.ComputedPortRoutes.ParentRef.Port,
		}
		destsSeen[prk] = struct{}{}
	}

	cid, err := cache.GetDecoded[*pbmesh.ComputedImplicitDestinations](
		rt.Cache,
		pbmesh.ComputedImplicitDestinationsType,
		"id",
		cidID,
	)
	if err != nil {
		return nil, err
	} else if cid == nil {
		return nil, nil
	}

	for _, dest := range cid.Data.GetDestinations() {
		serviceID := resource.IDFromReference(dest.DestinationRef)

		outDests, err := fetchSingleDestinationData(rt, workload.Id, mgwMode, serviceID, dest.DestinationPorts, nil)
		if err != nil {
			return nil, err
		} else if len(outDests) == 0 {
			continue // skip
		}

		for _, od := range outDests {
			// If it's already in destinations, ignore it.
			portName := od.ComputedPortRoutes.ParentRef.Port
			prk := PortReferenceKey{
				ReferenceKey: resource.NewReferenceKey(od.Service.Id),
				Port:         portName,
			}
			if _, ok := destsSeen[prk]; ok {
				continue
			}
			addToDestinations = append(addToDestinations, od)
		}
	}
	return addToDestinations, err
}

func fetchSingleDestinationData(
	rt controller.Runtime,
	workloadID *pbresource.ID,
	mgwMode pbmesh.MeshGatewayMode,
	serviceID *pbresource.ID,
	destPorts []string,
	explicitDest *pbmesh.Destination,
) ([]*intermediateTypes.Destination, error) {
	assertResourceType(pbcatalog.ServiceType, serviceID.Type)

	if explicitDest != nil {
		// Force this input regardless of what was asked.
		destPorts = []string{explicitDest.DestinationPort}
	}

	proxyTenancy := workloadID.GetTenancy()

	// Fetch Service
	svc, err := cache.GetDecoded[*pbcatalog.Service](rt.Cache, pbcatalog.ServiceType, "id", serviceID)
	if err != nil {
		return nil, err
	} else if svc == nil {
		// If the Service resource is not found, skip this destination.
		return nil, nil
	}

	// Check if this service is mesh-enabled. If not, update the status.
	if !svc.GetData().IsMeshEnabled() {
		// This error should not cause the execution to stop, as we want to make sure that this non-mesh destination
		// service gets removed from the proxy state.
		return nil, nil
	}

	// If this proxy is a part of this service, ignore it.
	if explicitDest == nil && isPartOfService(workloadID, svc) {
		return nil, nil
	}

	ports := make(map[string]struct{})
	for _, p := range destPorts {
		ports[p] = struct{}{}
	}

	// Remove any desired ports that do not exist on the service.
	for port := range ports {
		if svc.GetData().FindPortByID(port) == nil {
			delete(ports, port)
			continue
		}

		// No destination port should point to a port with "mesh" protocol,
		// so check if destination port has the mesh protocol and skip it if it does.
		if svc.GetData().FindPortByID(port).GetProtocol() == pbcatalog.Protocol_PROTOCOL_MESH {
			delete(ports, port)
			continue
		}
	}
	if len(ports) == 0 {
		return nil, nil
	}

	// Fetch ComputedRoutes.
	crID := resource.ReplaceType(pbmesh.ComputedRoutesType, serviceID)
	cr, err := cache.GetDecoded[*pbmesh.ComputedRoutes](rt.Cache, pbmesh.ComputedRoutesType, "id", crID)
	if err != nil {
		return nil, err
	} else if cr == nil {
		// This is required, so wait until it exists.
		return nil, nil
	}

	var dests []*intermediateTypes.Destination
	for port := range ports {
		portConfig, ok := cr.Data.PortedConfigs[port]
		if !ok {
			// This is required, so wait until it exists.
			delete(ports, port)
			continue
		}

		d := &intermediateTypes.Destination{
			Service: svc,
			// Copy this so we can mutate the targets.
			ComputedPortRoutes: proto.Clone(portConfig).(*pbmesh.ComputedPortRoutes),
		}

		if explicitDest != nil {
			// As Destinations resource contains a list of destinations,
			// we need to find the one that references our service and port.
			d.Explicit = explicitDest
		} else {
			// todo (ishustava): this should eventually grab virtual IPs resource.
			d.VirtualIPs = svc.Data.VirtualIps
		}

		// NOTE: we collect both DIRECT and INDIRECT target information here.
		for _, routeTarget := range d.ComputedPortRoutes.Targets {
			targetServiceID := resource.IDFromReference(routeTarget.BackendRef.Ref)

			// Fetch Service
			targetSvc, err := rt.Cache.Get(pbcatalog.ServiceType, "id", targetServiceID)
			if err != nil {
				return nil, err
			} else if targetSvc == nil {
				continue
			}

			// Gather all identities.
			ids := catalog.GetBoundIdentities(targetSvc)
			routeTarget.IdentityRefs = make([]*pbresource.Reference, len(ids))
			for i, id := range ids {
				routeTarget.IdentityRefs[i] = &pbresource.Reference{
					Name:    id,
					Tenancy: svc.Id.Tenancy,
				}
			}

			routeTarget.ServiceEndpointsRef = &pbproxystate.EndpointRef{
				Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, targetServiceID),
				MeshPort:  routeTarget.MeshPort,
				RoutePort: routeTarget.BackendRef.Port,
			}

			// If the target service is in a different partition and the mesh gateway mode is
			// "local" or "remote", use the ServiceEndpoints for the corresponding MeshGateway
			// instead of the ServiceEndpoints for the target service. The IdentityRefs on the
			// target will remain the same for TCP targets.
			//
			// TODO(nathancoleman) Consider cross-datacenter case as well
			if routeTarget.BackendRef.Ref.Tenancy.Partition != proxyTenancy.Partition {
				switch mgwMode {
				case pbmesh.MeshGatewayMode_MESH_GATEWAY_MODE_LOCAL:
					// Use ServiceEndpoints for the MeshGateway in the source service's partition
					routeTarget.ServiceEndpointsRef = &pbproxystate.EndpointRef{
						Id: &pbresource.ID{
							Type:    pbcatalog.ServiceEndpointsType,
							Name:    meshgateways.GatewayName,
							Tenancy: proxyTenancy,
						},
						MeshPort:  meshgateways.LANPortName,
						RoutePort: meshgateways.LANPortName,
					}
				case pbmesh.MeshGatewayMode_MESH_GATEWAY_MODE_REMOTE:
					// Use ServiceEndpoints for the MeshGateway in the target service's partition
					routeTarget.ServiceEndpointsRef = &pbproxystate.EndpointRef{
						Id: &pbresource.ID{
							Type:    pbcatalog.ServiceEndpointsType,
							Name:    meshgateways.GatewayName,
							Tenancy: targetServiceID.Tenancy,
						},
						MeshPort:  meshgateways.WANPortName,
						RoutePort: meshgateways.WANPortName,
					}
				}
			}
		}

		dests = append(dests, d)
	}

	return dests, nil
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
