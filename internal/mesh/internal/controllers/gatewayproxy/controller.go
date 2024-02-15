// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package gatewayproxy

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/apigateways"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/mapper"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/meshgateways"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ControllerName is the name for this controller. It's used for logging or status keys.
const (
	ControllerName         = "consul.io/gateway-proxy"
	GatewayKindMetadataKey = "gateway-kind"
)

// Controller is responsible for triggering reconciler for watched resources
func Controller(trustDomainFetcher sidecarproxy.TrustDomainFetcher, dc string, defaultAllow bool) *controller.Controller {
	// TODO NET-7016 Use caching functionality in NewController being implemented at time of writing
	// TODO NET-7017 Add the host of other types we should watch
	// TODO NET-7565: Add watch for serviceTypes across partitions
	return controller.NewController(ControllerName, pbmesh.ProxyStateTemplateType).
		WithWatch(pbcatalog.WorkloadType, dependency.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithWatch(pbmesh.ComputedProxyConfigurationType, dependency.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithWatch(pbmulticluster.ComputedExportedServicesType, mapper.AllMeshGatewayWorkloadsInPartition).
		WithWatch(pbmesh.TCPRouteType, mapper.APIGatewaysInParentRefs).
		WithReconciler(&reconciler{
			dc:             dc,
			defaultAllow:   defaultAllow,
			getTrustDomain: trustDomainFetcher,
		})
}

// reconciler is responsible for managing the ProxyStateTemplate for all
// gateway types: mesh, api (future) and terminating (future).
type reconciler struct {
	dc             string
	defaultAllow   bool
	getTrustDomain sidecarproxy.TrustDomainFetcher
}

// Reconcile is responsible for creating and updating the pbmesh.ProxyStateTemplate
// for all gateway types. Since the ProxyStateTemplates managed here will always have
// an owner reference pointing to the corresponding pbmesh.MeshGateway, deletion is
// left to the garbage collector.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID)
	rt.Logger.Trace("reconciling proxy state template")

	// Instantiate a data fetcher to fetch all reconciliation data.
	dataFetcher := fetcher.New(rt.Client)

	workloadID := resource.ReplaceType(pbcatalog.WorkloadType, req.ID)
	workload, err := dataFetcher.FetchWorkload(ctx, workloadID)
	if err != nil {
		rt.Logger.Error("error reading the associated workload", "error", err)
		return err
	}

	if workload == nil {
		rt.Logger.Trace("workload doesn't exist; skipping reconciliation", "workload", workloadID)
		// Workload no longer exists, let garbage collector clean up
		return nil
	}

	switch workload.Metadata[GatewayKindMetadataKey] {
	case meshgateways.GatewayKind:
		rt.Logger.Trace("workload is a mesh-gateway; reconciling", "workload", workloadID, "workloadData", workload.Data)
		return r.reconcileMeshGatewayProxyState(ctx, dataFetcher, workload, rt, req)
	case apigateways.GatewayKind:
		rt.Logger.Trace("workload is a api-gateway; reconciling", "workload", workloadID, "workloadData", workload.Data)
		return r.reconcileAPIGatewayProxyState(ctx, dataFetcher, workload, rt, req)
	default:
		rt.Logger.Trace("workload is not a gateway; skipping reconciliation", "workload", workloadID)
		return nil
	}
}

func (r *reconciler) reconcileAPIGatewayProxyState(ctx context.Context, dataFetcher *fetcher.Fetcher, workload *resource.DecodedResource[*pbcatalog.Workload], rt controller.Runtime, req controller.Request) error {
	proxyStateTemplate, err := dataFetcher.FetchProxyStateTemplate(ctx, req.ID)
	if err != nil {
		rt.Logger.Error("error reading proxy state template", "error", err)
		return nil
	}

	if proxyStateTemplate == nil {
		req.ID.Uid = ""
		rt.Logger.Trace("proxy state template for this gateway doesn't yet exist; generating a new one")
	}

	gwID := &pbresource.ID{
		Name:    workload.Data.Identity,
		Type:    pbmesh.APIGatewayType,
		Tenancy: workload.Id.Tenancy,
	}

	apiGateway, err := dataFetcher.FetchAPIGateway(ctx, gwID)
	if err != nil {
		rt.Logger.Error("error reading the associated api gateway", "error", err)
		return err
	}

	if apiGateway == nil {
		rt.Logger.Trace("api gateway doesn't exist; skipping reconciliation", "apiGatewayID", gwID)
		return nil
	}

	allTCPRoutes, err := dataFetcher.FetchAllTCPRoutes(ctx, req.ID.Tenancy)
	if err != nil {
		rt.Logger.Error("error reading the associated tcp routes", "error", err)
		return err
	}

	if len(allTCPRoutes) == 0 {
		rt.Logger.Trace("no tcp routes found for this gateway", "apiGatewayID", gwID)
	}

	services := make([]*pbcatalog.Service, 0)

	tcpRoutesReferencingGateway := make([]*pbmesh.TCPRoute, 0)
	for _, tcpRoute := range allTCPRoutes {
		for _, parentRef := range tcpRoute.Data.ParentRefs {
			if resource.EqualReference(parentRef.Ref, resource.ReferenceFromReferenceOrID(apiGateway.Id)) {
				tcpRoutesReferencingGateway = append(tcpRoutesReferencingGateway, tcpRoute.Data)
				for _, rule := range tcpRoute.Data.Rules {
					for _, backendRef := range rule.BackendRefs {
						service, err := dataFetcher.FetchService(ctx, resource.IDFromReference(backendRef.BackendRef.Ref))
						if err != nil {
							rt.Logger.Error("error reading the associated service", "error", err)
							return err
						}

						if service == nil {
							rt.Logger.Error("service was nil", "serviceID", resource.IDFromReference(backendRef.BackendRef.Ref))
							return nil
						}
						services = append(services, service.Data)
					}
				}
			}
		}
	}

	trustDomain, err := r.getTrustDomain()
	if err != nil {
		rt.Logger.Error("error fetching trust domain to compute proxy state template", "error", err)
		return err
	}

	gw := apiGateway.Data

	newPST := builder.NewAPIGWProxyStateTemplateBuilder(workload, services, tcpRoutesReferencingGateway, gw, rt.Logger, dataFetcher, r.dc, trustDomain).Build()

	proxyTemplateData, err := anypb.New(newPST)
	if err != nil {
		rt.Logger.Error("error creating proxy state template data", "error", err)
		return err
	}

	// If we're not creating a new PST and the generated one matches the existing one, nothing to do
	if proxyStateTemplate != nil && proto.Equal(proxyStateTemplate.Data, newPST) {
		rt.Logger.Trace("no changes to existing proxy state template")
		return nil
	}

	rt.Logger.Trace("updating proxy state template")

	// Write the created/updated ProxyStateTemplate with Workload as owner
	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:       req.ID,
			Metadata: map[string]string{"gateway-kind": workload.Metadata["gateway-kind"]},
			Owner:    workload.Resource.Id,
			Data:     proxyTemplateData,
		},
	})

	if err != nil {
		rt.Logger.Error("error writing proxy state template", "error", err)
		return err
	}

	return nil
}

func (r *reconciler) reconcileMeshGatewayProxyState(ctx context.Context, dataFetcher *fetcher.Fetcher, workload *resource.DecodedResource[*pbcatalog.Workload], rt controller.Runtime, req controller.Request) error {
	proxyStateTemplate, err := dataFetcher.FetchProxyStateTemplate(ctx, req.ID)
	if err != nil {
		rt.Logger.Error("error reading proxy state template", "error", err)
		return nil
	}

	if proxyStateTemplate == nil {
		req.ID.Uid = ""
		rt.Logger.Trace("proxy state template for this gateway doesn't yet exist; generating a new one")
	}

	exportedServicesID := &pbresource.ID{
		Name: "global",
		Tenancy: &pbresource.Tenancy{
			Partition: req.ID.Tenancy.Partition,
		},
		Type: pbmulticluster.ComputedExportedServicesType,
	}

	// This covers any incoming requests from outside my partition to services inside my partition
	var exportedServices []*pbmulticluster.ComputedExportedService
	dec, err := dataFetcher.FetchComputedExportedServices(ctx, exportedServicesID)
	if err != nil {
		rt.Logger.Error("error reading the associated exported services", "error", err)
	} else if dec == nil {
		rt.Logger.Error("exported services was nil")
	} else {
		exportedServices = dec.Data.Services
	}

	// This covers any incoming requests from inside my partition to services outside my partition
	meshGateways, err := dataFetcher.FetchMeshGateways(ctx, &pbresource.Tenancy{
		Partition: acl.WildcardPartitionName,
	})
	if err != nil {
		rt.Logger.Warn("error reading the associated mesh gateways", "error", err)
	}

	var remoteGatewayIDs []*pbresource.ID
	for _, meshGateway := range meshGateways {
		// If this is the mesh gateway in my local partition + datacenter, skip
		if meshGateway.Id.Tenancy.Partition != req.ID.Tenancy.Partition {
			remoteGatewayIDs = append(remoteGatewayIDs, meshGateway.Id)
		}
	}

	trustDomain, err := r.getTrustDomain()
	if err != nil {
		rt.Logger.Error("error fetching trust domain to compute proxy state template", "error", err)
		return err
	}

	newPST := builder.NewMeshGWProxyStateTemplateBuilder(workload, exportedServices, rt.Logger, dataFetcher, r.dc, trustDomain, remoteGatewayIDs).Build()

	proxyTemplateData, err := anypb.New(newPST)
	if err != nil {
		rt.Logger.Error("error creating proxy state template data", "error", err)
		return err
	}

	// If we're not creating a new PST and the generated one matches the existing one, nothing to do
	if proxyStateTemplate != nil && proto.Equal(proxyStateTemplate.Data, newPST) {
		rt.Logger.Trace("no changes to existing proxy state template")
		return nil
	}

	rt.Logger.Trace("updating proxy state template")

	// Write the created/updated ProxyStateTemplate with MeshGateway owner
	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:       req.ID,
			Metadata: map[string]string{"gateway-kind": workload.Metadata["gateway-kind"]},
			Owner:    workload.Resource.Id,
			Data:     proxyTemplateData,
		},
	})

	if err != nil {
		rt.Logger.Error("error writing proxy state template", "error", err)
		return err
	}

	return nil
}
