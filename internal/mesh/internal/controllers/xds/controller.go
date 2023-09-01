// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	proxysnapshot "github.com/hashicorp/consul/internal/mesh/proxy-snapshot"
	proxytracker "github.com/hashicorp/consul/internal/mesh/proxy-tracker"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const ControllerName = "consul.io/xds-controller"

func Controller(mapper *bimapper.Mapper, updater ProxyUpdater, fetcher TrustBundleFetcher) controller.Controller {
	//if mapper == nil || updater == nil || fetcher == nil {
	if mapper == nil || fetcher == nil {
		panic("mapper, updater and fetcher are required")
	}

	return controller.ForType(types.ProxyStateTemplateType).
		WithWatch(catalog.ServiceEndpointsType, mapper.MapLink).
		WithPlacement(controller.PlacementEachServer).
		WithReconciler(&xdsReconciler{bimapper: mapper, updater: updater, fetchTrustBundle: fetcher})
}

type xdsReconciler struct {
	bimapper         *bimapper.Mapper
	updater          ProxyUpdater
	fetchTrustBundle TrustBundleFetcher
}

type TrustBundleFetcher func() (*pbproxystate.TrustBundle, error)

// ProxyUpdater is an interface that defines the ability to push proxy updates to the updater
// and also check its connectivity to the server.
type ProxyUpdater interface {
	// PushChange allows pushing a computed ProxyState to xds for xds resource generation to send to a proxy.
	PushChange(id *pbresource.ID, snapshot proxysnapshot.ProxySnapshot) error

	// ProxyConnectedToServer returns whether this id is connected to this server.
	ProxyConnectedToServer(id *pbresource.ID) bool
}

func (r *xdsReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	rt.Logger.Trace("reconciling  proxy state template", "id", req.ID)

	// Get the ProxyStateTemplate.
	proxyStateTemplate, err := getProxyStateTemplate(ctx, rt, req.ID)
	if err != nil {
		rt.Logger.Error("error reading proxy state template", "error", err)
		return err
	}

	if proxyStateTemplate == nil || proxyStateTemplate.Template == nil || !r.updater.ProxyConnectedToServer(req.ID) {
		rt.Logger.Trace("proxy state template has been deleted or this controller is not responsible for this proxy state template", "id", req.ID)

		// If the proxy state was deleted, we should remove references to it in the mapper.
		r.bimapper.UntrackItem(req.ID)

		return nil
	}

	var (
		statusCondition *pbresource.Condition
		pstResource     *pbresource.Resource
	)
	pstResource = proxyStateTemplate.Resource

	// Initialize the ProxyState endpoints map.
	if proxyStateTemplate.Template.ProxyState == nil {
		rt.Logger.Error("proxy state was missing from proxy state template")
		// Set the status.
		statusCondition = status.ConditionRejectedNilProxyState(status.KeyFromID(req.ID))
		status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

		return err
	}

	// TODO: Fetch trust bundles for all peers when peering is supported.
	trustBundle, err := r.fetchTrustBundle()
	if err != nil {
		rt.Logger.Error("error fetching root trust bundle", "error", err)
		// Set the status.
		statusCondition = status.ConditionRejectedTrustBundleFetchFailed(status.KeyFromID(req.ID))
		status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)
		return err
	}

	if proxyStateTemplate.Template.ProxyState.TrustBundles == nil {
		proxyStateTemplate.Template.ProxyState.TrustBundles = make(map[string]*pbproxystate.TrustBundle)
	}
	// TODO: Figure out the correct key for the default trust bundle.
	proxyStateTemplate.Template.ProxyState.TrustBundles["local"] = trustBundle

	if proxyStateTemplate.Template.ProxyState.Endpoints == nil {
		proxyStateTemplate.Template.ProxyState.Endpoints = make(map[string]*pbproxystate.Endpoints)
	}

	// Iterate through the endpoint references.
	// For endpoints, the controller should:
	//  1. Resolve ServiceEndpoint references
	//  2. Translate them into pbproxystate.Endpoints
	//  3. Add the pbproxystate.Endpoints to the ProxyState endpoints map.
	//  4. Track relationships between ProxyState and ServiceEndpoints, such that we can look up ServiceEndpoints and
	//  figure out which ProxyStates are associated with it (for mapping watches) and vice versa (for looking up
	//  references). The bimapper package is useful for tracking these relationships.
	endpointReferencesMap := proxyStateTemplate.Template.RequiredEndpoints
	var endpointsInProxyStateTemplate []resource.ReferenceOrID
	for xdsClusterName, endpointRef := range endpointReferencesMap {

		// Step 1: Resolve the reference by looking up the ServiceEndpoints.
		// serviceEndpoints will not be nil unless there is an error.
		serviceEndpoints, err := getServiceEndpoints(ctx, rt, endpointRef.Id)
		if err != nil {
			rt.Logger.Error("error reading service endpoint", "id", endpointRef.Id, "error", err)
			// Set the status.
			statusCondition = status.ConditionRejectedErrorReadingEndpoints(status.KeyFromID(endpointRef.Id), err.Error())
			status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

			return err
		}

		// Step 2: Translate it into pbproxystate.Endpoints.
		psEndpoints, err := generateProxyStateEndpoints(serviceEndpoints, endpointRef.Port)
		if err != nil {
			rt.Logger.Error("error translating service endpoints to proxy state endpoints", "endpoint", endpointRef.Id, "error", err)

			// Set the status.
			statusCondition = status.ConditionRejectedCreatingProxyStateEndpoints(status.KeyFromID(endpointRef.Id), err.Error())
			status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

			return err
		}

		// Step 3: Add the endpoints to ProxyState.
		proxyStateTemplate.Template.ProxyState.Endpoints[xdsClusterName] = psEndpoints

		// Track all the endpoints that are used by this ProxyStateTemplate, so we can use this for step 4.
		endpointResourceRef := resource.Reference(endpointRef.Id, "")
		endpointsInProxyStateTemplate = append(endpointsInProxyStateTemplate, endpointResourceRef)

	}

	// Step 4: Track relationships between ProxyStateTemplates and ServiceEndpoints.
	r.bimapper.TrackItem(req.ID, endpointsInProxyStateTemplate)

	computedProxyState := proxyStateTemplate.Template.ProxyState

	err = r.updater.PushChange(req.ID, &proxytracker.ProxyState{ProxyState: computedProxyState})
	if err != nil {
		// Set the status.
		statusCondition = status.ConditionRejectedPushChangeFailed(status.KeyFromID(req.ID))
		status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)
		return err
	}

	// Set the status.
	statusCondition = status.ConditionAccepted()
	status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)
	return nil
}

func resourceIdToReference(id *pbresource.ID) *pbresource.Reference {
	ref := &pbresource.Reference{
		Name:    id.GetName(),
		Type:    id.GetType(),
		Tenancy: id.GetTenancy(),
	}
	return ref
}
