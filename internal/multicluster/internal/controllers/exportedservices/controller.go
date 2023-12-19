// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
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

func Controller() *controller.Controller {

	return controller.NewController(ControllerName, pbmulticluster.ComputedExportedServicesType).
		WithWatch(pbmulticluster.ExportedServicesType, ReplaceTypeForComputedExportedServices()).
		WithWatch(pbcatalog.ServiceType, ReplaceTypeForComputedExportedServices()).
		WithWatch(pbmulticluster.NamespaceExportedServicesType, ReplaceTypeForComputedExportedServices()).
		WithWatch(pbmulticluster.PartitionExportedServicesType, ReplaceTypeForComputedExportedServices()).
		WithReconciler(&reconciler{})
}

type reconciler struct{}

type serviceExports struct {
	ref        *pbresource.Reference
	partitions map[string]struct{}
	peers      map[string]struct{}
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
	builder := newExportedServicesBuilder()

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
				builder.track(id, es.Data.Consumers)
			}
		}
	}

	for _, nes := range namespaceExportedServices {
		for _, svc := range services {
			if svc.Id.Tenancy.Namespace != nes.Id.Tenancy.Namespace {
				continue
			}
			builder.track(svc.Id, nes.Data.Consumers)
		}
	}

	for _, pes := range partitionedExportedServices {
		for _, svc := range services {
			builder.track(svc.Id, pes.Data.Consumers)
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

type exportedServicesBuilder struct {
	data map[resource.ReferenceKey]*serviceExports
}

func newExportedServicesBuilder() *exportedServicesBuilder {
	return &exportedServicesBuilder{
		data: make(map[resource.ReferenceKey]*serviceExports),
	}
}

func (b *exportedServicesBuilder) track(id *pbresource.ID, consumers []*pbmulticluster.ExportedServicesConsumer) error {
	key := resource.NewReferenceKey(id)
	exports, ok := b.data[key]

	if !ok {
		exports = &serviceExports{
			ref:        resource.Reference(id, ""),
			partitions: make(map[string]struct{}),
			peers:      make(map[string]struct{}),
		}
		b.data[key] = exports
	}

	for _, c := range consumers {
		switch v := c.ConsumerTenancy.(type) {
		case *pbmulticluster.ExportedServicesConsumer_Peer:
			exports.peers[v.Peer] = struct{}{}
		case *pbmulticluster.ExportedServicesConsumer_Partition:
			exports.partitions[v.Partition] = struct{}{}
		case *pbmulticluster.ExportedServicesConsumer_SamenessGroup:
			// TODO do we currently validate that sameness groups can't be set?
			return fmt.Errorf("unexpected export to sameness group %q", v.SamenessGroup)
		}
	}

	return nil
}

func (b *exportedServicesBuilder) build() *pbmulticluster.ComputedExportedServices {
	if len(b.data) == 0 {
		return nil
	}

	ces := &pbmulticluster.ComputedExportedServices{
		Consumers: make([]*pbmulticluster.ComputedExportedService, 0, len(b.data)),
	}

	for _, svc := range sortRefValue(b.data) {
		consumers := make([]*pbmulticluster.ComputedExportedServicesConsumer, 0, len(svc.peers)+len(svc.partitions))

		for _, peer := range sortKeys(svc.peers) {
			consumers = append(consumers, &pbmulticluster.ComputedExportedServicesConsumer{
				ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
					Peer: peer,
				},
			})
		}

		for _, partition := range sortKeys(svc.partitions) {
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

func sortKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortRefValue(m map[resource.ReferenceKey]*serviceExports) []*serviceExports {
	vals := make([]*serviceExports, 0, len(m))
	for _, val := range m {
		vals = append(vals, val)
	}
	sort.Slice(vals, func(i, j int) bool {
		return resource.ReferenceToString(vals[i].ref) < resource.ReferenceToString(vals[j].ref)
	})
	return vals
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
