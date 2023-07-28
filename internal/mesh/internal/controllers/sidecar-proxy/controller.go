// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sidecar_proxy

import (
	"context"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/mapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// ControllerName is the name for this controller. It's used for logging or status keys.
const ControllerName = "consul.io/sidecar-proxy-controller"

type TrustDomainFetcher func() (string, error)

func Controller(cache *cache.Cache, mapper *mapper.Mapper, trustDomainFetcher TrustDomainFetcher) controller.Controller {
	if cache == nil || mapper == nil || trustDomainFetcher == nil {
		panic("cache, mapper and trust domain fetcher are required")
	}

	return controller.ForType(types.ProxyStateTemplateType).
		WithWatch(catalog.ServiceEndpointsType, mapper.MapServiceEndpointsToProxyStateTemplate).
		WithWatch(types.UpstreamsType, mapper.MapDestinationsToProxyStateTemplate).
		WithReconciler(&reconciler{cache: cache, getTrustDomain: trustDomainFetcher})
}

type reconciler struct {
	cache          *cache.Cache
	getTrustDomain TrustDomainFetcher
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	rt.Logger.Trace("reconciling proxy state template", "id", req.ID)

	// Instantiate a data fetcher to fetch all reconciliation data.
	dataFetcher := fetcher.Fetcher{Client: rt.Client, Cache: r.cache}

	// Check if the apiWorkload exists.
	workloadID := resource.ReplaceType(catalog.WorkloadType, req.ID)
	workload, err := dataFetcher.FetchWorkload(ctx, resource.ReplaceType(catalog.WorkloadType, req.ID))
	if err != nil {
		rt.Logger.Error("error reading the associated workload", "error", err)
		return err
	}
	if workload == nil {
		// If apiWorkload has been deleted, then return as ProxyStateTemplate should be cleaned up
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
		// If proxy state template has been deleted
		rt.Logger.Trace("proxy state template for this workload doesn't yet exist; generating a new one", "id", req.ID)
	}

	if !fetcher.IsMeshEnabled(workload.Workload.Ports) {
		// Skip non-mesh workloads.

		// If there's existing proxy state template, delete it.
		if proxyStateTemplate != nil {
			rt.Logger.Trace("deleting existing proxy state template because workload is no longer on the mesh", "id", req.ID)
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				rt.Logger.Error("error deleting existing proxy state template", "error", err)
				return err
			}

			// Remove it from cache.
			r.cache.DeleteSourceProxy(req.ID)
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

	b := builder.New(req.ID, workloadIdentityRefFromWorkload(workload), trustDomain).
		BuildLocalApp(workload.Workload)

	// Get all destinationsData.
	destinationsRefs := r.cache.DestinationsBySourceProxy(req.ID)
	destinationsData, statuses, err := dataFetcher.FetchDestinationsData(ctx, destinationsRefs)
	if err != nil {
		rt.Logger.Error("error fetching destinations for this proxy", "id", req.ID, "error", err)
		return err
	}

	b.BuildDestinations(destinationsData)

	newProxyTemplate := b.Build()

	if proxyStateTemplate == nil || !proto.Equal(proxyStateTemplate.Tmpl, newProxyTemplate) {
		proxyTemplateData, err := anypb.New(newProxyTemplate)
		if err != nil {
			rt.Logger.Error("error creating proxy state template data", "error", err)
			return err
		}
		rt.Logger.Trace("updating proxy state template", "id", req.ID)
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
		rt.Logger.Trace("proxy state template data has not changed, skipping update", "id", req.ID)
	}

	// Update any statuses.
	for _, status := range statuses {
		updatedStatus := &pbresource.Status{
			ObservedGeneration: status.Generation,
		}
		updatedStatus.Conditions = status.Conditions
		// If the status is unchanged then we should return and avoid the unnecessary write
		if !resource.EqualStatus(status.OldStatus[ControllerName], updatedStatus, false) {
			rt.Logger.Trace("updating status", "id", status.ID)
			_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
				Id:     status.ID,
				Key:    ControllerName,
				Status: updatedStatus,
			})
			if err != nil {
				rt.Logger.Error("error writing new status", "id", status.ID, "error", err)
				return err
			}
		}
	}
	return nil
}

func workloadIdentityRefFromWorkload(w *intermediate.Workload) *pbresource.Reference {
	return &pbresource.Reference{
		Name:    w.Workload.Identity,
		Tenancy: w.Resource.Id.Tenancy,
	}
}
