package gatewayproxy

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ControllerName is the name for this controller. It's used for logging or status keys.
const ControllerName = "consul.io/gateway-proxy-controller"

// Controller is responsible for triggering reconciler for watched resources
func Controller(cache *cache.Cache) controller.Controller {
	// TODO Add the host of other types we should watch
	return controller.ForType(pbmesh.ProxyStateTemplateType).
		WithWatch(pbcatalog.ServiceType, controller.ReplaceType(pbmesh.ProxyStateTemplateType)).
		WithReconciler(&reconciler{
			cache: cache,
		})
}

// reconciler is responsible for managing the ProxyStateTemplate for all
// gateway types: mesh, api (future) and terminating (future).
type reconciler struct {
	cache *cache.Cache
}

// Reconcile is responsible for creating and updating the pbmesh.ProxyStateTemplate
// for all gateway types. Since the ProxyStateTemplates managed here will always have
// an owner reference pointing to the corresponding pbmesh.MeshGateway, deletion is
// left to the garbage collector.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)
	rt.Logger.Trace("reconciling proxy state template")

	var gatewayType *pbresource.Type

	// If the workload is not for a xGateway, let the sidecarproxy reconciler handle it
	workloadID := resource.ReplaceType(pbcatalog.WorkloadType, req.ID)
	res, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: workloadID})
	if err != nil || res.Resource == nil || res.Resource.Id == nil {
		rt.Logger.Error("error reading the associated workload", "error", err)
		return err
	} else {
		if gatewayKind := res.Resource.Metadata["gateway-kind"]; gatewayKind == "" {
			rt.Logger.Trace("workload is not a gateway; skipping reconciliation", "workload", workloadID)
			return nil
		}
	}

	// Instantiate a data fetcher to fetch all reconciliation data.
	dataFetcher := fetcher.New(rt.Client, r.cache)

	// Check if the gateway exists.
	// TODO Switch fetch method based on gatewayType
	gatewayID := resource.ReplaceType(gatewayType, req.ID)
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
		rt.Logger.Trace("proxy state template for this gateway doesn't yet exist; generating a new one")
	}

	if proxyStateTemplate == nil {
		req.ID.Uid = ""
	}

	proxyTemplateData, err := anypb.New(proxyStateTemplate)
	if err != nil {
		rt.Logger.Error("error creating proxy state template data", "error", err)
		return err
	}
	rt.Logger.Trace("updating proxy state template")

	// Write the created/updated ProxyStateTemplate with MeshGateway owner
	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:    req.ID,
			Owner: resource.ReplaceType(gatewayType, req.ID),
			Data:  proxyTemplateData,
		},
	})
	if err != nil {
		rt.Logger.Error("error writing proxy state template", "error", err)
		return err
	}

	return nil
}
