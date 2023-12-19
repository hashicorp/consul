// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxyconfiguration

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/workloadselectionmapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const ControllerName = "consul.io/proxy-configuration-controller"

func Controller(proxyConfigMapper *workloadselectionmapper.Mapper[*pbmesh.ProxyConfiguration]) *controller.Controller {
	if proxyConfigMapper == nil {
		panic("proxy config mapper is required")
	}

	return controller.NewController(ControllerName, pbmesh.ComputedProxyConfigurationType).
		WithWatch(pbmesh.ProxyConfigurationType, proxyConfigMapper.MapToComputedType).
		WithWatch(pbcatalog.WorkloadType, dependency.ReplaceType(pbmesh.ComputedProxyConfigurationType)).
		WithReconciler(&reconciler{proxyConfigMapper: proxyConfigMapper})
}

type reconciler struct {
	proxyConfigMapper *workloadselectionmapper.Mapper[*pbmesh.ProxyConfiguration]
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("controller", ControllerName, "id", req.ID)

	// Look up the associated workload.
	workloadID := resource.ReplaceType(pbcatalog.WorkloadType, req.ID)
	workload, err := resource.GetDecodedResource[*pbcatalog.Workload](ctx, rt.Client, workloadID)
	if err != nil {
		rt.Logger.Error("error fetching workload", "error", err)
		return err
	}

	// If workload is not found, the decoded resource will be nil.
	if workload == nil {
		// When workload is not there, we don't need to manually delete the resource
		// because it is owned by the workload. In this case, we skip reconcile
		// because there's nothing for us to do.
		rt.Logger.Trace("the corresponding workload does not exist", "id", workloadID)
		return nil
	}

	// Get existing ComputedProxyConfiguration resource (if any).
	cpc, err := resource.GetDecodedResource[*pbmesh.ComputedProxyConfiguration](ctx, rt.Client, req.ID)
	if err != nil {
		rt.Logger.Error("error fetching ComputedProxyConfiguration", "error", err)
		return err
	}

	// If workload is not on the mesh, we need to delete the resource and return
	// as for non-mesh workloads there should be no proxy configuration.
	if !workload.GetData().IsMeshEnabled() {
		rt.Logger.Trace("workload is not on the mesh, skipping reconcile and deleting any corresponding ComputedProxyConfiguration", "id", workloadID)

		// Delete CPC only if it exists.
		if cpc != nil {
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				// If there's an error deleting CPC, we want to re-trigger reconcile again.
				rt.Logger.Error("error deleting ComputedProxyConfiguration", "error", err)
				return err
			}
		}

		// Otherwise, return as there's nothing else for us to do.
		return nil
	}

	// Now get any proxy configurations IDs that we have in the cache that have selectors matching the name
	// of this CPC (name-aligned with the workload).
	proxyCfgIDs := r.proxyConfigMapper.IDsForWorkload(req.ID)
	rt.Logger.Trace("cached proxy cfg IDs", "ids", proxyCfgIDs)

	decodedProxyCfgs, err := r.fetchProxyConfigs(ctx, rt.Client, proxyCfgIDs, workload)
	if err != nil {
		rt.Logger.Error("error fetching proxy configurations", "error", err)
		return err
	}

	// If after fetching, we don't have any proxy configs, we need to skip reconcile and delete the resource.
	if len(decodedProxyCfgs) == 0 {
		rt.Logger.Trace("found no proxy configurations associated with this workload")

		if cpc != nil {
			rt.Logger.Trace("deleting ComputedProxyConfiguration")
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				// If there's an error deleting CPC, we want to re-trigger reconcile again.
				rt.Logger.Error("error deleting ComputedProxyConfiguration", "error", err)
				return err
			}
		}

		return nil
	}

	// Next, we need to sort configs so that we can resolve conflicts.
	sortedProxyCfgs := SortProxyConfigurations(decodedProxyCfgs, req.ID.GetName())

	mergedProxyCfg := &pbmesh.ProxyConfiguration{}
	// Walk sorted configs in reverse order so that the ones that take precedence
	// do not overwrite the ones that don't.
	for i := len(sortedProxyCfgs) - 1; i >= 0; i-- {
		proto.Merge(mergedProxyCfg, sortedProxyCfgs[i].GetData())
	}

	newCPCData := &pbmesh.ComputedProxyConfiguration{
		DynamicConfig:   mergedProxyCfg.GetDynamicConfig(),
		BootstrapConfig: mergedProxyCfg.GetBootstrapConfig(),
	}

	// Lastly, write the resource.
	if cpc == nil || !proto.Equal(cpc.GetData(), newCPCData) {
		rt.Logger.Trace("writing new ComputedProxyConfiguration")

		// First encode the endpoints data as an Any type.
		cpcDataAsAny, err := anypb.New(newCPCData)
		if err != nil {
			rt.Logger.Error("error marshalling latest ComputedProxyConfiguration", "error", err)
			return err
		}

		_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
			Resource: &pbresource.Resource{
				Id:    req.ID,
				Owner: workloadID,
				Data:  cpcDataAsAny,
			},
		})
		if err != nil {
			rt.Logger.Error("error writing latest ComputedProxyConfiguration", "error", err)
			return err
		}
	}

	return nil
}

func (r *reconciler) fetchProxyConfigs(
	ctx context.Context,
	client pbresource.ResourceServiceClient,
	proxyCfgIds []*pbresource.ID,
	workload *types.DecodedWorkload,
) ([]*types.DecodedProxyConfiguration, error) {
	var decoded []*types.DecodedProxyConfiguration
	for _, id := range proxyCfgIds {
		res, err := resource.GetDecodedResource[*pbmesh.ProxyConfiguration](ctx, client, id)
		if err != nil {
			return nil, err
		}
		if res == nil || res.GetResource() == nil || res.GetData() == nil {
			// If resource is not found, we should untrack it.
			r.proxyConfigMapper.UntrackID(id)
			continue
		}

		if res.Data.Workloads.Filter != "" {
			match, err := resource.FilterMatchesResourceMetadata(workload.Resource, res.Data.Workloads.Filter)
			if err != nil {
				return nil, fmt.Errorf("error checking selector filters: %w", err)
			}
			if !match {
				continue
			}
		}

		decoded = append(decoded, res)
	}

	return decoded, nil
}
