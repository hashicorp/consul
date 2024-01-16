// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	expanderTypes "github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices/expander/types"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ControllerName = "consul.io/exported-services"
)

type ExportedServicesSamenessGroupExpander interface {
	// Expand resolves a sameness group into peers and partition and returns
	// them as individual slices
	//
	// It also returns back the list of sameness group names that can't be resolved.
	Expand([]*pbmulticluster.ExportedServicesConsumer, map[string][]*pbmulticluster.SamenessGroupMember) (*expanderTypes.ExpandedConsumers, error)

	// List returns the list of sameness groups present in a given partition
	List(context.Context, controller.Runtime, controller.Request) ([]*types.DecodedSamenessGroup, error)
}

func Controller(expander ExportedServicesSamenessGroupExpander) *controller.Controller {
	if expander == nil {
		panic("No sameness group expander was provided to the ExportedServiceController constructor")
	}

	ctrl := controller.NewController(ControllerName, pbmulticluster.ComputedExportedServicesType).
		WithWatch(pbmulticluster.ExportedServicesType, ReplaceTypeForComputedExportedServices()).
		WithWatch(pbcatalog.ServiceType, ReplaceTypeForComputedExportedServices()).
		WithWatch(pbmulticluster.NamespaceExportedServicesType, ReplaceTypeForComputedExportedServices()).
		WithWatch(pbmulticluster.PartitionExportedServicesType, ReplaceTypeForComputedExportedServices()).
		WithReconciler(&reconciler{samenessGroupExpander: expander})

	return registerEnterpriseResourcesWatchers(ctrl)
}

type reconciler struct {
	samenessGroupExpander ExportedServicesSamenessGroupExpander
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID)
	rt.Logger.Trace("reconciling exported services")
	exportedServices, err := resource.ListDecodedResource[*pbmulticluster.ExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: req.ID.Tenancy.Partition,
			PeerName:  resource.DefaultPeerName,
		},
		Type: pbmulticluster.ExportedServicesType,
	})
	if err != nil {
		rt.Logger.Error("error getting exported services", "error", err)
		return err
	}
	namespaceExportedServices, err := resource.ListDecodedResource[*pbmulticluster.NamespaceExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: req.ID.Tenancy.Partition,
			PeerName:  resource.DefaultPeerName,
		},
		Type: pbmulticluster.NamespaceExportedServicesType,
	})
	if err != nil {
		rt.Logger.Error("error getting namespace exported services", "error", err)
		return err
	}
	partitionedExportedServices, err := resource.ListDecodedResource[*pbmulticluster.PartitionExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Partition: req.ID.Tenancy.Partition,
			PeerName:  resource.DefaultPeerName,
		},
		Type: pbmulticluster.PartitionExportedServicesType,
	})
	if err != nil {
		rt.Logger.Error("error getting partitioned exported services", "error", err)
		return err
	}
	oldComputedExportedService, err := getOldComputedExportedService(ctx, rt, req)
	if err != nil {
		return err
	}
	if len(exportedServices) == 0 && len(namespaceExportedServices) == 0 && len(partitionedExportedServices) == 0 {
		if oldComputedExportedService.GetResource() != nil {
			rt.Logger.Trace("deleting computed exported services")
			if err := deleteResource(ctx, rt, oldComputedExportedService.GetResource()); err != nil {
				rt.Logger.Error("error deleting computed exported service", "error", err)
				return err
			}
		}
		return nil
	}
	namespace := getNamespaceForServices(exportedServices, namespaceExportedServices, partitionedExportedServices)
	services, err := resource.ListDecodedResource[*pbcatalog.Service](ctx, rt.Client, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Namespace: namespace,
			Partition: req.ID.Tenancy.Partition,
			PeerName:  resource.DefaultPeerName,
		},
		Type: pbcatalog.ServiceType,
	})
	if err != nil {
		rt.Logger.Error("error getting services", "error", err)
		return err
	}

	samenessGroups, err := r.samenessGroupExpander.List(ctx, rt, req)
	if err != nil {
		rt.Logger.Error("failed to fetch sameness groups", err)
		return err
	}

	builder := newExportedServicesBuilder(r.samenessGroupExpander, samenessGroups)

	svcs := make(map[resource.ReferenceKey]struct{}, len(services))
	for _, svc := range services {
		svcs[resource.NewReferenceKey(svc.Id)] = struct{}{}
	}

	for _, es := range exportedServices {
		for _, svc := range es.Data.Services {
			id := &pbresource.ID{
				Type:    pbcatalog.ServiceType,
				Tenancy: es.Id.Tenancy,
				Name:    svc,
			}
			if _, ok := svcs[resource.NewReferenceKey(id)]; ok {
				if err := builder.track(id, es.Data.Consumers); err != nil {
					rt.Logger.Error("error tracking service for exported service",
						"exported_service", es.Id.Name,
						"service", id.Name,
						"error", err,
					)
					return err
				}
			}
		}
	}

	for _, nes := range namespaceExportedServices {
		for _, svc := range services {
			if svc.Id.Tenancy.Namespace != nes.Id.Tenancy.Namespace {
				continue
			}
			if err := builder.track(svc.Id, nes.Data.Consumers); err != nil {
				rt.Logger.Error("error tracking service for namespace exported service",
					"exported_service", nes.Id.Name,
					"service", svc.Id.Name,
					"error", err,
				)
				return err
			}
		}
	}

	for _, pes := range partitionedExportedServices {
		for _, svc := range services {
			if err := builder.track(svc.Id, pes.Data.Consumers); err != nil {
				rt.Logger.Error("error tracking service for partition exported service",
					"exported_service", pes.Id.Name,
					"service", svc.Id.Name,
					"error", err,
				)
				return err
			}
		}
	}
	newComputedExportedService := builder.build()

	if oldComputedExportedService.GetResource() != nil && newComputedExportedService == nil {
		rt.Logger.Trace("deleting computed exported services")
		if err := deleteResource(ctx, rt, oldComputedExportedService.GetResource()); err != nil {
			rt.Logger.Error("error deleting computed exported service", "error", err)
			return err
		}
		return nil
	}
	if proto.Equal(newComputedExportedService, oldComputedExportedService.GetData()) {
		rt.Logger.Trace("skip writing computed exported services")
		return nil
	}
	newComputedExportedServiceData, err := anypb.New(newComputedExportedService)
	if err != nil {
		rt.Logger.Error("error marshalling latest computed exported service", "error", err)
		return err
	}

	rt.Logger.Trace("writing computed exported services")
	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:    req.ID,
			Owner: nil,
			Data:  newComputedExportedServiceData,
		},
	})
	if err != nil {
		rt.Logger.Error("error writing computed exported service", "error", err)
		return err
	}
	return nil
}

func ReplaceTypeForComputedExportedServices() controller.DependencyMapper {
	return func(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		return []controller.Request{
			{
				ID: &pbresource.ID{
					Type: pbmulticluster.ComputedExportedServicesType,
					Tenancy: &pbresource.Tenancy{
						Partition: res.Id.Tenancy.Partition,
					},
					Name: "global",
				},
			},
		}, nil
	}
}

func getOldComputedExportedService(ctx context.Context, rt controller.Runtime, req controller.Request) (*resource.DecodedResource[*pbmulticluster.ComputedExportedServices], error) {
	computedExpSvcID := &pbresource.ID{
		Name: types.ComputedExportedServicesName,
		Type: pbmulticluster.ComputedExportedServicesType,
		Tenancy: &pbresource.Tenancy{
			Partition: req.ID.Tenancy.Partition,
		},
	}
	computedExpSvcRes, err := resource.GetDecodedResource[*pbmulticluster.ComputedExportedServices](ctx, rt.Client, computedExpSvcID)
	if err != nil {
		rt.Logger.Error("error fetching computed exported service", "error", err)
		return nil, err
	}
	return computedExpSvcRes, nil
}

func getNamespaceForServices(exportedServices []*types.DecodedExportedServices, namespaceExportedServices []*types.DecodedNamespaceExportedServices, partitionedExportedServices []*types.DecodedPartitionExportedServices) string {
	if len(partitionedExportedServices) > 0 {
		return storage.Wildcard
	}
	resources := []*pbresource.Resource{}
	for _, exp := range exportedServices {
		resources = append(resources, exp.GetResource())
	}
	for _, exp := range namespaceExportedServices {
		resources = append(resources, exp.GetResource())
	}
	return getNamespace(resources)
}

func getNamespace(resources []*pbresource.Resource) string {
	if len(resources) == 0 {
		// We shouldn't ever hit this.
		panic("resources cannot be empty")
	}

	namespace := resources[0].Id.Tenancy.Namespace
	for _, res := range resources[1:] {
		if res.Id.Tenancy.Namespace != namespace {
			return storage.Wildcard
		}
	}
	return namespace
}

func deleteResource(ctx context.Context, rt controller.Runtime, resource *pbresource.Resource) error {
	_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{
		Id:      resource.GetId(),
		Version: resource.GetVersion(),
	})
	if err != nil {
		return err
	}
	return nil
}
