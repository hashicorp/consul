// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"context"
	"slices"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/indexers"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func mapExportedServices(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return []controller.Request{
		{
			ID: &pbresource.ID{
				Type: pbmulticluster.ComputedExportedServicesType,
				Tenancy: &pbresource.Tenancy{
					Partition: res.Id.Tenancy.Partition,
				},
				Name: types.ComputedExportedServicesName,
			},
		},
	}, nil
}

func transformExportedServices(mapper controller.DependencyMapper) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		reqs, err := mapper(ctx, rt, res)
		if err != nil {
			return nil, err
		}

		for _, req := range reqs {
			req.ID = &pbresource.ID{
				Type: pbmulticluster.ComputedExportedServicesType,
				Tenancy: &pbresource.Tenancy{
					Partition: req.ID.Tenancy.Partition,
				},
				Name: types.ComputedExportedServicesName,
			}
		}
		return reqs, err
	}
}

// Controller creates a controller for automatic ComputedExportedServices management
func Controller() controller.Controller {
	return controller.ForType(pbmulticluster.ComputedExportedServicesType).
		WithWatch(pbmulticluster.ExportedServicesType, mapExportedServices).
		WithIndex(pbmulticluster.ExportedServicesType, "services", indexers.ServiceIndexer()).
		WithWatch(pbmulticluster.NamespaceExportedServicesType, mapExportedServices).
		WithIndex(pbmulticluster.NamespaceExportedServicesType, "services", indexers.ServiceIndexer()).
		WithWatch(pbmulticluster.PartitionExportedServicesType, mapExportedServices).
		WithIndex(pbmulticluster.PartitionExportedServicesType, "services", indexers.ServiceIndexer()).
		WithWatch(pbcatalog.ServiceType, transformExportedServices(controller.MultiMapper(
			controller.CacheParentsMapper(pbmulticluster.PartitionExportedServicesType, "services"),
			controller.CacheParentsMapper(pbmulticluster.NamespaceExportedServicesType, "services"),
			controller.CacheParentsMapper(pbmulticluster.ExportedServicesType, "services"),
		))).
		WithReconciler(&reconciler{})
}

type reconciler struct {
}

// Reconcile will reconcile one ComputedExportedServices in response to some event.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", "exported-services")

	existing, err := resource.GetDecodedResource[*pbmulticluster.ComputedExportedServices](ctx, rt.Client, req.ID)
	if err != nil {
		rt.Logger.Error("error retrieving existing computed exported services", "error", err)
	}

	partitionExports, err := resource.ListDecodedResource[*pbmulticluster.PartitionExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Type:    pbmulticluster.PartitionExportedServicesType,
		Tenancy: req.ID.Tenancy,
	})

	if err != nil {
		rt.Logger.Error("error retrieving partition exported services", "error", err)
		return err
	}

	namespaceExports, err := resource.ListDecodedResource[*pbmulticluster.NamespaceExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Type:    pbmulticluster.NamespaceExportedServicesType,
		Tenancy: req.ID.Tenancy,
	})

	if err != nil {
		rt.Logger.Error("error retrieving namespace exported service", "error", err)
		return err
	}

	serviceExports, err := resource.ListDecodedResource[*pbmulticluster.ExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Type:    pbmulticluster.ExportedServicesType,
		Tenancy: req.ID.Tenancy,
	})

	if err != nil {
		rt.Logger.Error("error retrieving exported services", "error", err)
		return err
	}

	services, err := resource.ListDecodedResource[*pbcatalog.Service](ctx, rt.Client, &pbresource.ListRequest{
		Type:    pbcatalog.ServiceType,
		Tenancy: req.ID.Tenancy,
	})

	if err != nil {
		rt.Logger.Error("error retrieving services", "error", err)
		return err
	}

	if len(partitionExports) == 0 && len(namespaceExports) == 0 && len(serviceExports) == 0 {
		if existing == nil {
			return nil
		}

		_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
		if err != nil {
			rt.Logger.Error("failed to delete computed exported services", "error", err)
		}
		return err
	}

	exports := newExportTracker()

	for _, partitionExport := range partitionExports {
		for _, svc := range services {
			exports.trackServiceConsumers(svc.Id, partitionExport.Data.Consumers)
		}
	}

	for _, namespaceExport := range namespaceExports {
		for _, svc := range services {
			if svc.Id.Tenancy.Namespace != namespaceExport.Id.Tenancy.Namespace {
				continue
			}

			exports.trackServiceConsumers(svc.Id, namespaceExport.Data.Consumers)
		}
	}

	for _, serviceExport := range serviceExports {
		for _, svc := range serviceExport.Data.Services {
			svcId := &pbresource.ID{
				Type:    pbcatalog.ServiceType,
				Tenancy: serviceExport.Id.Tenancy,
				Name:    svc,
			}

			exports.trackServiceConsumers(svcId, serviceExport.Data.Consumers)
		}
	}

	ces := exports.getComputedExportedServices()

	if existing != nil && proto.Equal(existing.Data, ces) {
		rt.Logger.Trace("no new computed exported services")
		return nil
	}

	cesAny, err := anypb.New(ces)
	if err != nil {
		rt.Logger.Error("error marshalling latest computed exported services", "error", err)
		return err
	}

	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:   req.ID,
			Data: cesAny,
		},
	})

	if err != nil {
		rt.Logger.Error("error writing new computed exported services", "error", err)
		return err
	} else {
		rt.Logger.Trace("new computed exported services were successfully written")
	}

	return nil
}

type exportTracker struct {
	services map[resource.ReferenceKey]*exportServiceTracker
}

type exportServiceTracker struct {
	ref        *pbresource.Reference
	partitions map[string]struct{}
	peers      map[string]struct{}
}

func newExportTracker() *exportTracker {
	return &exportTracker{
		services: make(map[resource.ReferenceKey]*exportServiceTracker),
	}
}

func (t *exportTracker) trackServiceConsumers(svcID *pbresource.ID, consumers []*pbmulticluster.ExportedServicesConsumer) {
	key := resource.NewReferenceKey(svcID)

	svcTracker, ok := t.services[key]
	if !ok {
		svcTracker = &exportServiceTracker{
			ref:        resource.Reference(svcID, ""),
			partitions: make(map[string]struct{}),
			peers:      make(map[string]struct{}),
		}
		t.services[key] = svcTracker
	}

	for _, consumer := range consumers {
		switch v := consumer.ConsumerTenancy.(type) {
		case *pbmulticluster.ExportedServicesConsumer_Peer:
			svcTracker.peers[v.Peer] = struct{}{}
		case *pbmulticluster.ExportedServicesConsumer_Partition:
			svcTracker.partitions[v.Partition] = struct{}{}
			// TODO - update to expand sameness groups
		}
	}
}

func (t *exportTracker) getComputedExportedServices() *pbmulticluster.ComputedExportedServices {
	ces := &pbmulticluster.ComputedExportedServices{
		Consumers: make([]*pbmulticluster.ComputedExportedService, 0, len(t.services)),
	}

	for _, svc := range t.services {
		consumers := make([]*pbmulticluster.ComputedExportedServicesConsumer, 0, len(svc.peers)+len(svc.partitions))

		for _, peer := range sortedKeys(svc.peers) {
			consumers = append(consumers, &pbmulticluster.ComputedExportedServicesConsumer{
				ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
					Peer: peer,
				},
			})
		}

		for _, partition := range sortedKeys(svc.partitions) {
			consumers = append(consumers, &pbmulticluster.ComputedExportedServicesConsumer{
				ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Partition{
					Partition: partition,
				},
			})
		}

		ces.Consumers = append(ces.Consumers, &pbmulticluster.ComputedExportedService{
			TargetRef: svc.ref,
			Consumers: consumers,
		})
	}

	return ces
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	slices.Sort(keys)
	return keys
}
