package types

import (
	"github.com/hashicorp/consul/internal/resource"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func createAggregatedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        GroupName,
				GroupVersion: VersionV1Alpha1,
				Kind:         multiclusterv1alpha1.ExportedServicesKind,
			},
			Tenancy: resource.DefaultPartitionedTenancy(),
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validAggregateServicesResourceWithPeer() *multiclusterv1alpha1.AggregateExportedServices {
	return &multiclusterv1alpha1.AggregateExportedServices{
		Consumers: []*multiclusterv1alpha1.AggregateExportedService{
			{
				Consumers: []*multiclusterv1alpha1.AggregateExportedServicesConsumer{
					{
						Name:            "service1",
						Namespace:       "ns1",
						ConsumerTenancy: &multiclusterv1alpha1.AggregateExportedServicesConsumer_Peer{Peer: "peer"},
					},
				},
			},
		},
	}
}

func validAggregateServicesResourceWithPartition() *multiclusterv1alpha1.AggregateExportedServices {
	return &multiclusterv1alpha1.AggregateExportedServices{
		Consumers: []*multiclusterv1alpha1.AggregateExportedService{
			{
				Consumers: []*multiclusterv1alpha1.AggregateExportedServicesConsumer{
					{
						Name:            "service1",
						Namespace:       "ns1",
						ConsumerTenancy: &multiclusterv1alpha1.AggregateExportedServicesConsumer_Partition{Partition: "partition"},
					},
				},
			},
		},
	}
}

func TestCreateAggregatedExportedServicesResourceWithPeer(t *testing.T) {
	createAggregatedServicesResource(t, validAggregateServicesResourceWithPeer())
}

func TestCreateAggregatedExportedServicesResourceWithNamespace(t *testing.T) {
	createAggregatedServicesResource(t, validAggregateServicesResourceWithPartition())
}
