package types

import (
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func createPartitionExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: multiclusterv1alpha1.PartitionExportedServicesType,
			Name: "partition-exported-services-1",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validPartitionExportedServicesWithPeer() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := make([]*multiclusterv1alpha1.ExportedServicesConsumer, 1)
	consumers[0] = new(multiclusterv1alpha1.ExportedServicesConsumer)
	consumers[0].ConsumerTenancy = &multiclusterv1alpha1.ExportedServicesConsumer_Peer{Peer: "peer"}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithPartition() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := make([]*multiclusterv1alpha1.ExportedServicesConsumer, 1)
	consumers[0] = new(multiclusterv1alpha1.ExportedServicesConsumer)
	consumers[0].ConsumerTenancy = &multiclusterv1alpha1.ExportedServicesConsumer_Partition{Partition: "partition"}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithSamenessGroup() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := make([]*multiclusterv1alpha1.ExportedServicesConsumer, 1)
	consumers[0] = new(multiclusterv1alpha1.ExportedServicesConsumer)
	consumers[0].ConsumerTenancy = &multiclusterv1alpha1.ExportedServicesConsumer_SamenessGroup{SamenessGroup: "sameness_group"}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func TestValidatePartitionExportedServicesWithPeer_Ok(t *testing.T) {
	res := createPartitionExportedServicesResource(t, validPartitionExportedServicesWithPeer())
	var resDecoded multiclusterv1alpha1.PartitionExportedServices
	err := res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, "peer", resDecoded.Consumers[0].GetPeer())
}

func TestValidatePartitionExportedServicesWithPartition_Ok(t *testing.T) {
	res := createPartitionExportedServicesResource(t, validPartitionExportedServicesWithPartition())
	var resDecoded multiclusterv1alpha1.PartitionExportedServices
	err := res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, "partition", resDecoded.Consumers[0].GetPartition())
}

func TestValidatePartitionExportedServicesWithSamenessGroup_Ok(t *testing.T) {
	res := createPartitionExportedServicesResource(t, validPartitionExportedServicesWithSamenessGroup())
	var resDecoded multiclusterv1alpha1.PartitionExportedServices
	err := res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, "sameness_group", resDecoded.Consumers[0].GetSamenessGroup())
}
