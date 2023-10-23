package exportedservices

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/storage"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Controller creates a controller for automatic ComputedExportedServices management for
// updates to WorkloadIdentity or TrafficPermission resources.
func Controller() controller.Controller {

	return controller.ForType(pbmulticluster.ComputedExportedServicesType).
		WithWatch(pbmulticluster.ExportedServicesType, ReplaceTypeForComputedTrafficPermissions(pbmulticluster.ComputedExportedServicesType)).
		WithWatch(pbcatalog.ServiceType, ReplaceTypeForComputedTrafficPermissions(pbmulticluster.ComputedExportedServicesType)).
		WithWatch(pbmulticluster.NamespaceExportedServicesType, ReplaceTypeForComputedTrafficPermissions(pbmulticluster.ComputedExportedServicesType)).
		WithWatch(pbmulticluster.PartitionExportedServicesType, ReplaceTypeForComputedTrafficPermissions(pbmulticluster.ComputedExportedServicesType)).
		WithReconciler(&reconciler{})
}

type reconciler struct{}

// Reconcile will reconcile one ComputedTrafficPermission (CTP) in response to some event.
// Events include adding, modifying or deleting a WorkloadIdentity or TrafficPermission.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
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
	servicesResp, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: req.ID.Tenancy.Partition,
		},
		Type: pbcatalog.ServiceType,
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
			Namespace: storage.Wildcard,
			Partition: req.ID.Tenancy.Partition,
		},
		Type: pbmulticluster.PartitionExportedServicesType,
	})
	if err != nil {
		return err
	}

	targetRefs := map[*pbresource.ID]map[*pbmulticluster.ComputedExportedServicesConsumer]struct{}{}
	for _, resource := range exportedServices.Resources {
		var expService pbmulticluster.ExportedServices
		if err := resource.Data.UnmarshalTo(&expService); err != nil {
			rt.Logger.Error("error unmarshalling computedExportedService data", "error", err)
			return err
		}
		computedServiceConsumers := []*pbmulticluster.ComputedExportedServicesConsumer{}
		for _, consumer := range expService.Consumers {
			switch consumer.GetConsumerTenancy().(type) {
			case *pbmulticluster.ExportedServicesConsumer_Partition:
				computedServiceConsumers = append(computedServiceConsumers, &pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Partition{
					Partition: consumer.GetPartition(),
				}})
			case *pbmulticluster.ExportedServicesConsumer_Peer:
				computedServiceConsumers = append(computedServiceConsumers, &pbmulticluster.ComputedExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
					Peer: consumer.GetPeer(),
				}})
			}
		}
		for _, svc := range expService.Services {
			targetRefID := &pbresource.ID{
				Name:    svc,
				Type:    pbcatalog.ServiceType,
				Tenancy: resource.Id.Tenancy,
			}
			for _, computerSvcConsumer := range computedServiceConsumers {
				if _, ok := targetRefs[targetRefID]; !ok {
					targetRefs[targetRefID] = map[*pbmulticluster.ComputedExportedServicesConsumer]struct{}{
						computerSvcConsumer: struct{}{},
					}
					continue
				}
				targetRefs[targetRefID][computerSvcConsumer] = struct{}{}
			}
		}

	}
	for _, resource := range namespaceExportedServices.Resources {
		var namespaceExpSvc pbmulticluster.NamespaceExportedServices
		if err := resource.Data.UnmarshalTo(&namespaceExpSvc); err != nil {
			rt.Logger.Error("error unmarshalling NamespaceExportedServices data", "error", err)
			return err
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
			var namespaceExpSvc pbmulticluster.NamespaceExportedServices
			if err := resource.Data.UnmarshalTo(&namespaceExpSvc); err != nil {
				rt.Logger.Error("error unmarshalling NamespaceExportedServices data", "error", err)
				return err
			}
		}
		// for _, consumer := range namespaceExpSvc.Consumers {

		// }
	}
	// for _, resource := range partitionedExportedServices.Resources {
	// 	targetRefs[resource.Id] = struct{}{}
	// }

	newComputedExportedService := pbmulticluster.ComputedExportedServices{
		Consumers: []*pbmulticluster.ComputedExportedService{{}},
	}
	for targetRef, consumers := range targetRefs {
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
		//TODO: do something here
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}
	res := rsp.Resource
	var computedExportedServices pbmulticluster.ComputedExportedServices
	if err := res.Data.UnmarshalTo(&computedExportedServices); err != nil {
		rt.Logger.Error("error unmarshalling computedExportedService data", "error", err)
		return err
	}
	shouldUpdate := false
	for _, computedExpSvc := range computedExportedServices.Consumers {
		if _, ok := targetRefs[computedExpSvc.TargetRef]; !ok {
			shouldUpdate = true
			break
		}
		for _, consumer := range computedExpSvc.Consumers {
			for expSvc := range targetRefs[computedExpSvc.TargetRef] {
				if expSvc.ConsumerTenancy != consumer.ConsumerTenancy {
					shouldUpdate = true
					break
				}
			}
		}
	}
	if !shouldUpdate {
		return nil
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

func ReplaceTypeForComputedTrafficPermissions(desiredType *pbresource.Type) controller.DependencyMapper {
	return func(_ context.Context, _ Runtime, res *pbresource.Resource) ([]Request, error) {
		return []Request{
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
