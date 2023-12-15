// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/fetcher"
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
	cache *cache.Cache,
	trustDomainFetcher TrustDomainFetcher,
	dc string,
	defaultAllow bool,
) *controller.Controller {
	if cache == nil || trustDomainFetcher == nil {
		panic("cache and trust domain fetcher are required")
	}

	/*
			Workload                   <align>   PST
			ComputedDestinations       <align>   PST(==Workload)
			ComputedDestinations       <contain> Service(destinations)
			ComputedProxyConfiguration <align>   PST(==Workload)
			ComputedRoutes             <align>   Service(upstream)
			ComputedRoutes             <contain> Service(disco)
		    ComputedTrafficPermissions <align>   WorkloadIdentity
		    Workload                   <contain> WorkloadIdentity

			These relationships then dictate the following reconcile logic.

			controller: read workload for PST
			controller: read previous PST
			controller: read ComputedProxyConfiguration for Workload
			controller: read ComputedDestinations for workload to walk explicit upstreams
		    controller: read ComputedTrafficPermissions for workload using workload.identity field.
			<EXPLICIT-for-each>
				fetcher: read Service(Destination)
				fetcher: read ComputedRoutes
				<TARGET-for-each>
					fetcher: read ServiceEndpoints
				</TARGET-for-each>
			</EXPLICIT-for-each>
			<IMPLICIT>
				fetcher: list ALL ComputedRoutes
				<CR-for-each>
					fetcher: read Service(upstream)
					<TARGET-for-each>
						fetcher: read ServiceEndpoints
					</TARGET-for-each>
				</CR-for-each>
			</IMPLICIT>
	*/

	/*
			Which means for equivalence, the following mapper relationships should exist:

			Service:                    find destinations with Service; Recurse(ComputedDestinations);
		                                find ComputedRoutes with this in a Target or FailoverConfig; Recurse(ComputedRoutes)
			ComputedDestinations:       replace type CED=>PST
			ComputedProxyConfiguration: replace type CPC=>PST
			ComputedRoutes:             CR=>Service; find destinations with Service; Recurse(Destinations)
								        [implicit/temp]: trigger all
		    ComputedTrafficPermissions: find workloads in cache stored for this CTP=Workload, workloads=>PST reconcile requests
	*/

	return controller.NewController(ControllerName, pbmesh.ProxyStateTemplateType).
		WithWatch(pbcatalog.ServiceType, cache.MapService).
		WithWatch(pbcatalog.WorkloadType, dependency.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithWatch(pbmesh.ComputedExplicitDestinationsType, dependency.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithWatch(pbmesh.ComputedProxyConfigurationType, dependency.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithWatch(pbmesh.ComputedRoutesType, cache.MapComputedRoutes).
		WithWatch(pbauth.ComputedTrafficPermissionsType, cache.MapComputedTrafficPermissions).
		WithReconciler(&reconciler{
			cache:          cache,
			getTrustDomain: trustDomainFetcher,
			dc:             dc,
			defaultAllow:   defaultAllow,
		})
}

type reconciler struct {
	cache          *cache.Cache
	getTrustDomain TrustDomainFetcher
	defaultAllow   bool
	dc             string
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	rt.Logger.Trace("reconciling proxy state template")

	// Instantiate a data fetcher to fetch all reconciliation data.
	dataFetcher := fetcher.New(rt.Client, r.cache)

	// Check if the workload exists.
	workloadID := resource.ReplaceType(pbcatalog.WorkloadType, req.ID)
	workload, err := dataFetcher.FetchWorkload(ctx, workloadID)
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

	proxyStateTemplate, err := dataFetcher.FetchProxyStateTemplate(ctx, req.ID)
	if err != nil {
		rt.Logger.Error("error reading proxy state template", "error", err)
		return nil
	}

	if proxyStateTemplate == nil {
		// If proxy state template has been deleted, we will need to generate a new one.
		rt.Logger.Trace("proxy state template for this workload doesn't yet exist; generating a new one")
	}

	if !workload.GetData().IsMeshEnabled() {
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
	proxyCfg, err := dataFetcher.FetchComputedProxyConfiguration(ctx, req.ID)
	if err != nil {
		rt.Logger.Error("error fetching proxy and merging proxy configurations", "error", err)
		return err
	}

	trafficPermissions, err := dataFetcher.FetchComputedTrafficPermissions(ctx, computedTrafficPermissionsIDFromWorkload(workload))
	if err != nil {
		rt.Logger.Error("error fetching computed traffic permissions to compute proxy state template", "error", err)
		return err
	}

	var ctp *pbauth.ComputedTrafficPermissions
	if trafficPermissions != nil {
		ctp = trafficPermissions.Data
	}

	workloadPorts, err := r.workloadPortProtocolsFromService(ctx, dataFetcher, workload, rt.Logger)
	if err != nil {
		rt.Logger.Error("error determining workload ports", "error", err)
		return err
	}
	workloadDataWithInheritedPorts := proto.Clone(workload.Data).(*pbcatalog.Workload)
	workloadDataWithInheritedPorts.Ports = workloadPorts

	b := builder.New(req.ID, identityRefFromWorkload(workload), trustDomain, r.dc, r.defaultAllow, proxyCfg.GetData()).
		BuildLocalApp(workloadDataWithInheritedPorts, ctp)

	// Get all destinationsData.
	destinationsData, err := dataFetcher.FetchExplicitDestinationsData(ctx, req.ID)
	if err != nil {
		rt.Logger.Error("error fetching explicit destinations for this proxy", "error", err)
		return err
	}

	if proxyCfg.GetData() != nil && proxyCfg.GetData().IsTransparentProxy() {
		rt.Logger.Trace("transparent proxy is enabled; fetching implicit destinations")
		destinationsData, err = dataFetcher.FetchImplicitDestinationsData(ctx, req.ID, destinationsData)
		if err != nil {
			rt.Logger.Error("error fetching implicit destinations for this proxy", "error", err)
			return err
		}
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

func (r *reconciler) workloadPortProtocolsFromService(
	ctx context.Context,
	fetcher *fetcher.Fetcher,
	workload *types.DecodedWorkload,
	logger hclog.Logger,
) (map[string]*pbcatalog.WorkloadPort, error) {

	// Fetch all services for this workload.
	serviceIDs := r.cache.ServicesForWorkload(workload.GetResource().GetId())

	var services []*types.DecodedService

	for _, serviceID := range serviceIDs {
		svc, err := fetcher.FetchService(ctx, serviceID)
		if err != nil {
			return nil, err
		}

		// If service is not found, we should untrack it.
		if svc == nil {
			r.cache.UntrackService(serviceID)
			continue
		}

		services = append(services, svc)
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
		if len(serviceIDs) == 0 || len(services) == 0 {
			logger.Trace("found no services for this workload's port; using default TCP protocol", "port", portName)
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
			svcPort := svc.GetData().FindServicePort(portName)

			// If this service doesn't select this port, go to the next service.
			if svcPort == nil {
				continue
			}

			// Check for conflicts.
			// If protocols between services selecting this workload on this port do not match,
			// we use the default protocol (tcp) instead.
			if inheritedProtocol != pbcatalog.Protocol_PROTOCOL_UNSPECIFIED &&
				svcPort.GetProtocol() != inheritedProtocol {

				logger.Trace("found conflicting service protocols that select this workload port; using default TCP protocol", "port", portName)
				inheritedProtocol = pbcatalog.Protocol_PROTOCOL_TCP

				// We won't check any remaining services as there's already a conflict.
				break
			}

			inheritedProtocol = svcPort.GetProtocol()
		}

		// If after going through all services, we haven't found a protocol, use the default.
		if inheritedProtocol == pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
			logger.Trace("no services select this workload port; using default TCP protocol", "port", portName)
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

func identityRefFromWorkload(w *types.DecodedWorkload) *pbresource.Reference {
	return &pbresource.Reference{
		Name:    w.Data.Identity,
		Tenancy: w.Resource.Id.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}
}

func computedTrafficPermissionsIDFromWorkload(w *types.DecodedWorkload) *pbresource.ID {
	return &pbresource.ID{
		Type:    pbauth.ComputedTrafficPermissionsType,
		Name:    w.Data.Identity,
		Tenancy: w.Resource.Id.Tenancy,
	}
}
