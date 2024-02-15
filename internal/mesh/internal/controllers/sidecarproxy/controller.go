// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/catalog/workloadselector"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ControllerName is the name for this controller. It's used for logging or status keys.
const ControllerName = "consul.io/sidecar-proxy-controller"

type TrustDomainFetcher func() (string, error)

func Controller(
	trustDomainFetcher TrustDomainFetcher,
	dc string,
	defaultAllow bool,
) *controller.Controller {
	if trustDomainFetcher == nil {
		panic("trust domain fetcher is required")
	}

	/*
			Workload                     <align>   PST
			ComputedExplicitDestinations <align>   PST(==Workload)
			ComputedExplicitDestinations <contain> Service(destinations)
			ComputedProxyConfiguration   <align>   PST(==Workload)
			ComputedRoutes               <align>   Service(upstream)
			ComputedRoutes               <contain> Service(disco)
		    ComputedTrafficPermissions   <align>   WorkloadIdentity
		    Workload                     <contain> WorkloadIdentity
			ComputedImplicitDestinations <align>   WorkloadIdentity
			ComputedImplicitDestinations <contain> Service(destinations)

			These relationships then dictate the following reconcile logic.

			controller: read workload for PST
			controller: read previous PST
			controller: read ComputedProxyConfiguration for Workload
			controller: read ComputedExplicitDestinations for workload to walk explicit destinations
			controller: maybe read ComputedImplicitDestinations for workload.identity to walk implicit destinations
		    controller: read ComputedTrafficPermissions for workload using workload.identity field.
			<EXPLICIT_OR_IMPLICIT-for-each>
				fetcher: read Service(Destination)
				fetcher: read ComputedRoutes
				<TARGET-for-each>
					fetcher: read Service
				</TARGET-for-each>
			</EXPLICIT_OR_IMPLICIT-for-each>
	*/

	/*
			Which means for equivalence, the following mapper relationships should exist:

			Service:                      find destinations with Service; Recurse(ComputedXDestinations);
		                                  find ComputedRoutes with this in a Target or FailoverConfig; Recurse(ComputedRoutes)
			ComputedExplicitDestinations: replace type CED=>PST
			ComputedImplicitDestinations: replace type CID=>WI; find workload with this identity; replace type WRK=>PST
			ComputedProxyConfiguration:   replace type CPC=>PST
			ComputedRoutes:               CR=>Service; find destinations with Service; Recurse(Destinations)
								          [implicit/temp]: trigger all
		    ComputedTrafficPermissions:   find workloads in cache stored for this CTP=Workload, workloads=>PST reconcile requests
	*/

	// TODO(rb): ultimately needs some form of BoundReferences
	return controller.NewController(ControllerName, pbmesh.ProxyStateTemplateType).
		WithWatch(pbcatalog.ServiceType,
			MapService,
			serviceByWorkloadIdentityIndex,
			workloadselector.Index[*pbcatalog.Service](selectedWorkloadsIndexName),
		).
		WithWatch(pbcatalog.WorkloadType,
			dependency.ReplaceType(pbmesh.ProxyStateTemplateType),
			workloadByWorkloadIdentityIndex,
		).
		WithWatch(pbmesh.ComputedExplicitDestinationsType,
			dependency.ReplaceType(pbmesh.ProxyStateTemplateType),
			computedExplicitDestinationsByServiceIndex,
		).
		WithWatch(pbmesh.ComputedImplicitDestinationsType,
			MapComputedImplicitDestinations,
			computedImplicitDestinationsByServiceIndex,
		).
		WithWatch(pbmesh.ComputedProxyConfigurationType,
			dependency.ReplaceType(pbmesh.ProxyStateTemplateType),
		).
		WithWatch(pbmesh.ComputedRoutesType,
			MapComputedRoutes,
			computedRoutesByBackendServiceIndex,
		).
		WithWatch(pbauth.ComputedTrafficPermissionsType,
			MapComputedTrafficPermissions,
		).
		WithReconciler(&reconciler{
			getTrustDomain: trustDomainFetcher,
			dc:             dc,
			defaultAllow:   defaultAllow,
		})
}

type reconciler struct {
	getTrustDomain TrustDomainFetcher
	defaultAllow   bool
	dc             string
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	rt.Logger.Trace("reconciling proxy state template")

	// Check if the workload exists.
	workloadID := resource.ReplaceType(pbcatalog.WorkloadType, req.ID)
	workload, err := cache.GetDecoded[*pbcatalog.Workload](rt.Cache, pbcatalog.WorkloadType, "id", workloadID)
	if err != nil {
		rt.Logger.Error("error reading the associated workload", "error", err)
		return err
	}

	if workload == nil {
		// If workload has been deleted, then return as ProxyStateTemplate should be cleaned up
		// by the garbage collector because of the owner reference.
		rt.Logger.Trace("workload doesn't exist; skipping reconciliation", "workload", workloadID)
		return nil
	}

	// If the workload is for a xGateway, then do nothing + let the gatewayproxy controller reconcile it
	if gatewayKind, ok := workload.Metadata["gateway-kind"]; ok && gatewayKind != "" {
		rt.Logger.Trace("workload is a gateway; skipping reconciliation", "workload", workloadID, "gateway-kind", gatewayKind)
		return nil
	}

	proxyStateTemplate, err := cache.GetDecoded[*pbmesh.ProxyStateTemplate](rt.Cache, pbmesh.ProxyStateTemplateType, "id", req.ID)
	if err != nil {
		rt.Logger.Error("error reading proxy state template", "error", err)
		return nil
	}

	if proxyStateTemplate == nil {
		// If proxy state template has been deleted, we will need to generate a new one.
		rt.Logger.Trace("proxy state template for this workload doesn't yet exist; generating a new one")
	}

	if !workload.GetData().IsMeshEnabled() || workload.Data.Identity == "" {
		// Skip non-mesh workloads.

		// If there's existing proxy state template, delete it.
		if proxyStateTemplate != nil {
			rt.Logger.Trace("deleting existing proxy state template because workload is no longer on the mesh")
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				rt.Logger.Error("error deleting existing proxy state template", "error", err)
				return err
			}
		}
		rt.Logger.Trace("skipping proxy state template generation because workload is not on the mesh", "workload", workload.Resource.Id)
		return nil
	}

	// First get the trust domain.
	trustDomain, err := r.getTrustDomain()
	if err != nil {
		rt.Logger.Error("error fetching trust domain to compute proxy state template", "error", err)
		return err
	}

	// Fetch proxy configuration.
	cpcID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, req.ID)
	proxyCfg, err := cache.GetDecoded[*pbmesh.ComputedProxyConfiguration](rt.Cache, pbmesh.ComputedProxyConfigurationType, "id", cpcID)
	if err != nil {
		rt.Logger.Error("error fetching proxy and merging proxy configurations", "error", err)
		return err
	}
	// note the proxyCfg may be nil

	ctpID := &pbresource.ID{
		Type:    pbauth.ComputedTrafficPermissionsType,
		Name:    workload.Data.Identity,
		Tenancy: workload.Resource.Id.Tenancy,
	}
	trafficPermissions, err := cache.GetDecoded[*pbauth.ComputedTrafficPermissions](rt.Cache, pbauth.ComputedTrafficPermissionsType, "id", ctpID)
	if err != nil {
		rt.Logger.Error("error fetching computed traffic permissions to compute proxy state template", "error", err)
		return err
	}

	var ctp *pbauth.ComputedTrafficPermissions
	if trafficPermissions != nil {
		ctp = trafficPermissions.Data
	}

	workloadPorts, err := workloadPortProtocolsFromService(rt, workload)
	if err != nil {
		rt.Logger.Error("error determining workload ports", "error", err)
		return err
	}
	workloadDataWithInheritedPorts := proto.Clone(workload.Data).(*pbcatalog.Workload)
	workloadDataWithInheritedPorts.Ports = workloadPorts

	workloadIdentityRef := &pbresource.Reference{
		Name:    workload.Data.Identity,
		Tenancy: workload.Resource.Id.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}

	b := builder.New(
		req.ID,
		workloadIdentityRef,
		trustDomain,
		r.dc,
		r.defaultAllow,
		proxyCfg.GetData(),
	).
		BuildLocalApp(workloadDataWithInheritedPorts, ctp)

	mgwMode := pbmesh.MeshGatewayMode_MESH_GATEWAY_MODE_NONE
	if dynamicCfg := proxyCfg.GetData().GetDynamicConfig(); dynamicCfg != nil {
		mgwMode = dynamicCfg.GetMeshGatewayMode()
	}

	// Get all destinationsData.
	destinationsData, err := FetchUnifiedDestinationsData(
		ctx, rt, workload, mgwMode,
		proxyCfg.GetData().IsTransparentProxy())
	if err != nil {
		rt.Logger.Error("error fetching destinations for this proxy", "error", err)
		return err
	}

	b.BuildDestinations(destinationsData)

	newProxyTemplate := b.Build()

	if proxyStateTemplate == nil || !proto.Equal(proxyStateTemplate.Data, newProxyTemplate) {
		if proxyStateTemplate == nil {
			req.ID.Uid = ""
		}
		proxyTemplateData, err := anypb.New(newProxyTemplate)
		if err != nil {
			rt.Logger.Error("error creating proxy state template data", "error", err)
			return err
		}
		rt.Logger.Trace("updating proxy state template")
		_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
			Resource: &pbresource.Resource{
				Id:    req.ID,
				Owner: workload.Resource.Id,
				Data:  proxyTemplateData,
			},
		})
		if err != nil {
			rt.Logger.Error("error writing proxy state template", "error", err)
			return err
		}
	} else {
		rt.Logger.Trace("proxy state template data has not changed, skipping update")
	}

	return nil
}

func workloadPortProtocolsFromService(rt controller.Runtime, workload *types.DecodedWorkload) (map[string]*pbcatalog.WorkloadPort, error) {
	// Fetch all services for this workload.
	services, err := cache.ParentsDecoded[*pbcatalog.Service](rt.Cache, pbcatalog.ServiceType, selectedWorkloadsIndexName, workload.Id)
	if err != nil {
		return nil, err
	}

	// Now walk through all workload ports.
	// For ports that don't have a protocol explicitly specified, inherit it from the service.

	result := make(map[string]*pbcatalog.WorkloadPort)

	for portName, port := range workload.GetData().GetPorts() {
		if port.GetProtocol() != pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
			// Add any specified protocols as is.
			result[portName] = port
			continue
		}

		// Check if we have any service IDs or fetched services.
		if len(services) == 0 {
			rt.Logger.Trace("found no services for this workload's port; using default TCP protocol", "port", portName)
			result[portName] = &pbcatalog.WorkloadPort{
				Port:     port.GetPort(),
				Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
			}
			continue
		}

		// Otherwise, look for port protocol in the service.
		inheritedProtocol := pbcatalog.Protocol_PROTOCOL_UNSPECIFIED
		for _, svc := range services {
			// Find workload's port as the target port.
			svcPort := svc.Data.FindTargetPort(portName)

			// If this service doesn't select this port, go to the next service.
			if svcPort == nil {
				continue
			}

			// Check for conflicts.
			// If protocols between services selecting this workload on this port do not match,
			// we use the default protocol (tcp) instead.
			if inheritedProtocol != pbcatalog.Protocol_PROTOCOL_UNSPECIFIED &&
				svcPort.GetProtocol() != inheritedProtocol {

				rt.Logger.Trace("found conflicting service protocols that select this workload port; using default TCP protocol", "port", portName)
				inheritedProtocol = pbcatalog.Protocol_PROTOCOL_TCP

				// We won't check any remaining services as there's already a conflict.
				break
			}

			inheritedProtocol = svcPort.GetProtocol()
		}

		// If after going through all services, we haven't found a protocol, use the default.
		if inheritedProtocol == pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
			rt.Logger.Trace("no services select this workload port; using default TCP protocol", "port", portName)
			result[portName] = &pbcatalog.WorkloadPort{
				Port:     port.GetPort(),
				Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
			}
		} else {
			result[portName] = &pbcatalog.WorkloadPort{
				Port:     port.GetPort(),
				Protocol: inheritedProtocol,
			}
		}
	}

	return result, nil
}
