package exportedservices

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/storage"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Controller creates a controller for automatic ComputedExportedServices management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller() controller.Controller {

	return controller.ForType(pbmulticluster.ComputedExportedServicesType).
		WithWatch(pbmulticluster.ExportedServicesType, ReplaceTypeForComputedExportedServices(pbmulticluster.ComputedExportedServicesType)).
		WithWatch(pbcatalog.ServiceType, ReplaceTypeForComputedExportedServices(pbmulticluster.ComputedExportedServicesType)).
		WithWatch(pbmulticluster.NamespaceExportedServicesType, ReplaceTypeForComputedExportedServices(pbmulticluster.ComputedExportedServicesType)).
		WithWatch(pbmulticluster.PartitionExportedServicesType, ReplaceTypeForComputedExportedServices(pbmulticluster.ComputedExportedServicesType)).
		WithReconciler(&reconciler{})
}

type reconciler struct{}

// Reconcile will reconcile one ComputedTrafficPermission (CTP) in response to some event.
// Events include adding, modifying or deleting a WorkloadIdentity or TrafficPermission.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)
	exportedServices, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: req.ID.Tenancy.Partition,
		},
		Type: pbmulticluster.ExportedServicesType,
	})
	if err != nil {
		return err
	}
	namespaceExportedServices, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: req.ID.Tenancy.Partition,
		},
		Type: pbmulticluster.NamespaceExportedServicesType,
	})
	if err != nil {
		return err
	}
	partitionedExportedServices, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Partition: req.ID.Tenancy.Partition,
		},
		Type: pbmulticluster.PartitionExportedServicesType,
	})
	if err != nil {
		return err
	}
	fmt.Println("jjkh", len(exportedServices.Resources))
	targetRefs := map[*pbresource.Reference]map[*pbmulticluster.ComputedExportedServicesConsumer]struct{}{}
	for _, resource := range exportedServices.Resources {
		var expService pbmulticluster.ExportedServices
		if err := resource.Data.UnmarshalTo(&expService); err != nil {
			rt.Logger.Error("error unmarshalling computedExportedService data", "error", err)
			return err
		}
		computedServiceConsumers := map[*pbmulticluster.ComputedExportedServicesConsumer]struct{}{}
		for _, consumer := range expService.Consumers {
			switch consumer.GetConsumerTenancy().(type) {
			case *pbmulticluster.ExportedServicesConsumer_Partition:
				computedServiceConsumers[&pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Partition{
					Partition: consumer.GetPartition(),
				}}] = struct{}{}
			case *pbmulticluster.ExportedServicesConsumer_Peer:
				computedServiceConsumers[&pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
					Peer: consumer.GetPeer(),
				}}] = struct{}{}
			}
		}
		fmt.Println("jjk", computedServiceConsumers)
		for _, svc := range expService.Services {
			targetRefID := &pbresource.Reference{
				Name:    svc,
				Type:    pbcatalog.ServiceType,
				Tenancy: resource.Id.Tenancy,
			}
			if _, ok := targetRefs[targetRefID]; !ok {
				targetRefs[targetRefID] = computedServiceConsumers
				continue
			}
			for computedServiceConsumer := range computedServiceConsumers {
				targetRefs[targetRefID][computedServiceConsumer] = struct{}{}
			}

		}

	}
	for _, resource := range namespaceExportedServices.Resources {
		var namespaceExpSvc pbmulticluster.NamespaceExportedServices
		if err := resource.Data.UnmarshalTo(&namespaceExpSvc); err != nil {
			rt.Logger.Error("error unmarshalling NamespaceExportedServices data", "error", err)
			return err
		}
		computedServiceConsumers := map[*pbmulticluster.ComputedExportedServicesConsumer]struct{}{}
		for _, consumer := range namespaceExpSvc.Consumers {
			switch consumer.GetConsumerTenancy().(type) {
			case *pbmulticluster.ExportedServicesConsumer_Partition:
				computedServiceConsumers[&pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Partition{
					Partition: consumer.GetPartition(),
				}}] = struct{}{}
			case *pbmulticluster.ExportedServicesConsumer_Peer:
				computedServiceConsumers[&pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
					Peer: consumer.GetPeer(),
				}}] = struct{}{}
			}
		}
		servicesResp, err := rt.Client.List(ctx, &pbresource.ListRequest{
			Tenancy: &pbresource.Tenancy{
				Namespace: resource.Id.Tenancy.Namespace,
				Partition: req.ID.Tenancy.Partition,
			},
			Type: pbcatalog.ServiceType,
		})
		if err != nil {
			return err
		}
		for _, res := range servicesResp.Resources {
			ref := &pbresource.Reference{
				Name:    res.Id.Name,
				Type:    res.Id.Type,
				Tenancy: res.Id.Tenancy,
			}
			if _, ok := targetRefs[ref]; !ok {
				targetRefs[ref] = computedServiceConsumers
				continue
			}
			for computedServiceConsumer := range computedServiceConsumers {
				targetRefs[ref][computedServiceConsumer] = struct{}{}
			}
		}
	}
	for _, resource := range partitionedExportedServices.Resources {
		var partitionedExpSvc pbmulticluster.PartitionExportedServices
		if err := resource.Data.UnmarshalTo(&partitionedExpSvc); err != nil {
			rt.Logger.Error("error unmarshalling NamespaceExportedServices data", "error", err)
			return err
		}
		computedServiceConsumers := map[*pbmulticluster.ComputedExportedServicesConsumer]struct{}{}
		for _, consumer := range partitionedExpSvc.Consumers {
			switch consumer.GetConsumerTenancy().(type) {
			case *pbmulticluster.ExportedServicesConsumer_Partition:
				computedServiceConsumers[&pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Partition{
					Partition: consumer.GetPartition(),
				}}] = struct{}{}
			case *pbmulticluster.ExportedServicesConsumer_Peer:
				computedServiceConsumers[&pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
					Peer: consumer.GetPeer(),
				}}] = struct{}{}
			}
		}
		servicesResp, err := rt.Client.List(ctx, &pbresource.ListRequest{
			Tenancy: &pbresource.Tenancy{
				Namespace: storage.Wildcard,
				Partition: resource.Id.Tenancy.Partition,
			},
			Type: pbcatalog.ServiceType,
		})
		if err != nil {
			return err
		}
		for _, res := range servicesResp.Resources {
			ref := &pbresource.Reference{
				Name:    res.Id.Name,
				Type:    res.Id.Type,
				Tenancy: res.Id.Tenancy,
			}
			if _, ok := targetRefs[ref]; !ok {
				targetRefs[ref] = computedServiceConsumers
				continue
			}
			for computedServiceConsumer := range computedServiceConsumers {
				targetRefs[ref][computedServiceConsumer] = struct{}{}
			}
		}
	}
	newComputedExportedService := pbmulticluster.ComputedExportedServices{
		Consumers: []*pbmulticluster.ComputedExportedService{{}},
	}
	for targetRef, consumers := range targetRefs {
		fmt.Println("consumers", consumers)
		newComputedExportedService.Consumers = append(newComputedExportedService.Consumers, &pbmulticluster.ComputedExportedService{
			TargetRef: targetRef,
			Consumers: keys(consumers),
		})
	}
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name: "global",
			Type: pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{
				Partition: req.ID.Tenancy.Partition,
			},
		},
	})
	switch {
	case status.Code(err) == codes.NotFound:
		rt.Logger.Trace("comp exp service is not found")
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}
	var computedExportedServices pbmulticluster.ComputedExportedServices
	if rsp != nil && rsp.Resource != nil {
		if err := rsp.Resource.Data.UnmarshalTo(&computedExportedServices); err != nil {
			rt.Logger.Error("error unmarshalling computedExportedService data", "error", err)
			return err
		}
	}
	if proto.Equal(&newComputedExportedService, &computedExportedServices) {
		return nil
	}
	newComputedExportedServiceData, err := anypb.New(&newComputedExportedService)
	if err != nil {
		rt.Logger.Error("error marshalling latest endpoints", "error", err)
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
		rt.Logger.Error("error writing generated endpoints", "error", err)
		return err
	}
	return nil
}

func keys(m map[*pbmulticluster.ComputedExportedServicesConsumer]struct{}) []*pbmulticluster.ComputedExportedServicesConsumer {
	keys := make([]*pbmulticluster.ComputedExportedServicesConsumer, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func ReplaceTypeForComputedExportedServices(desiredType *pbresource.Type) controller.DependencyMapper {
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
