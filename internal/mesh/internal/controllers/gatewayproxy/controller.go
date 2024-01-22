// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package gatewayproxy

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ControllerName is the name for this controller. It's used for logging or status keys.
const ControllerName = "consul.io/gateway-proxy"

// Controller is responsible for triggering reconciler for watched resources
func Controller(cache *cache.Cache, trustDomainFetcher sidecarproxy.TrustDomainFetcher, dc string, defaultAllow bool) *controller.Controller {
	// TODO NET-7016 Use caching functionality in NewController being implemented at time of writing
	// TODO NET-7017 Add the host of other types we should watch
	return controller.NewController(ControllerName, pbmesh.ProxyStateTemplateType).
		WithWatch(pbcatalog.WorkloadType, dependency.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithWatch(pbmesh.ComputedProxyConfigurationType, dependency.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithReconciler(&reconciler{
			cache:          cache,
			dc:             dc,
			defaultAllow:   defaultAllow,
			getTrustDomain: trustDomainFetcher,
		})
}

// reconciler is responsible for managing the ProxyStateTemplate for all
// gateway types: mesh, api (future) and terminating (future).
type reconciler struct {
	cache          *cache.Cache
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
	dataFetcher := fetcher.New(rt.Client, r.cache)

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

	// If the workload is not for a xGateway, let the sidecarproxy reconciler handle it
	if gatewayKind := workload.Metadata["gateway-kind"]; gatewayKind == "" {
		rt.Logger.Trace("workload is not a gateway; skipping reconciliation", "workload", workloadID, "workloadData", workload.Data)
		return nil
	}

	// TODO NET-7014 Determine what gateway controls this workload
	// For now, we cheat by knowing the MeshGateway's name, type + tenancy ahead of time
	gatewayID := &pbresource.ID{
		Name:    "mesh-gateway",
		Type:    pbmesh.MeshGatewayType,
		Tenancy: resource.DefaultPartitionedTenancy(),
	}

	// Check if the gateway exists.
	gateway, err := dataFetcher.FetchMeshGateway(ctx, gatewayID)
	if err != nil {
		rt.Logger.Error("error reading the associated gateway", "error", err)
		return err
	}
	if gateway == nil {
		// If gateway has been deleted, then return as ProxyStateTemplate should be
		// cleaned up by the garbage collector because of the owner reference.
		rt.Logger.Trace("gateway doesn't exist; skipping reconciliation", "gateway", gatewayID)
		return nil
	}

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

	var exportedServices []*pbmulticluster.ComputedExportedService
	dec, err := dataFetcher.FetchExportedServices(ctx, exportedServicesID)
	if err != nil {
		rt.Logger.Error("error reading the associated exported services", "error", err)
	} else if dec == nil {
		rt.Logger.Error("exported services was nil")
	} else {
		exportedServices = dec.Data.Services
	}

	trustDomain, err := r.getTrustDomain()
	if err != nil {
		rt.Logger.Error("error fetching trust domain to compute proxy state template", "error", err)
		return err
	}

	newPST := builder.NewProxyStateTemplateBuilder(workload, exportedServices, rt.Logger, dataFetcher, r.dc, trustDomain).Build()

	proxyTemplateData, err := anypb.New(newPST)
	if err != nil {
		rt.Logger.Error("error creating proxy state template data", "error", err)
		return err
	}
	rt.Logger.Trace("updating proxy state template")

	// If we're not creating a new PST and the generated one matches the existing one, nothing to do
	if proxyStateTemplate != nil && proto.Equal(proxyStateTemplate.Data, newPST) {
		rt.Logger.Trace("no changes to existing proxy state template")
		return nil
	}

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
