package types

import (
	"errors"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func createComputedExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage, name string) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: multiclusterv1alpha1.ComputedExportedServicesType,
			Name: name,
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validComputedExportedServicesWithPeer() *multiclusterv1alpha1.ComputedExportedServices {
	consumers := make([]*multiclusterv1alpha1.ComputedExportedService, 1)
	consumers[0] = new(multiclusterv1alpha1.ComputedExportedService)
	var computedExportedServicePeer *multiclusterv1alpha1.ComputedExportedServicesConsumer_Peer
	computedExportedServicePeer = new(multiclusterv1alpha1.ComputedExportedServicesConsumer_Peer)
	computedExportedServicePeer.Peer = "peer"
	consumers[0].Consumers = []*multiclusterv1alpha1.ComputedExportedServicesConsumer{
		{
			ConsumerTenancy: computedExportedServicePeer,
		},
	}
	return &multiclusterv1alpha1.ComputedExportedServices{
		Consumers: consumers,
	}
}

func validComputedExportedServicesWithPartition() *multiclusterv1alpha1.ComputedExportedServices {
	consumers := make([]*multiclusterv1alpha1.ComputedExportedService, 1)
	consumers[0] = new(multiclusterv1alpha1.ComputedExportedService)
	var computedExportedServicePartition *multiclusterv1alpha1.ComputedExportedServicesConsumer_Partition
	computedExportedServicePartition = new(multiclusterv1alpha1.ComputedExportedServicesConsumer_Partition)
	computedExportedServicePartition.Partition = "partition"
	consumers[0].Consumers = []*multiclusterv1alpha1.ComputedExportedServicesConsumer{
		{
			ConsumerTenancy: computedExportedServicePartition,
		},
	}
	return &multiclusterv1alpha1.ComputedExportedServices{
		Consumers: consumers,
	}
}

func TestValidateComputedExportedServicesWithPeer_Ok(t *testing.T) {
	res := createComputedExportedServicesResource(t, validComputedExportedServicesWithPeer(), ComputedExportedServicesName)

	err := ValidateComputedExportedServices(res)
	require.NoError(t, err)

	var resDecoded multiclusterv1alpha1.ComputedExportedServices
	err = res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, 1, len(resDecoded.Consumers[0].Consumers))
	require.Equal(t, "peer", resDecoded.Consumers[0].Consumers[0].GetPeer())
}

func TestValidateComputedExportedServicesWithPartition_Ok(t *testing.T) {
	res := createComputedExportedServicesResource(t, validComputedExportedServicesWithPartition(), ComputedExportedServicesName)

	err := ValidateComputedExportedServices(res)
	require.NoError(t, err)

	var resDecoded multiclusterv1alpha1.ComputedExportedServices
	err = res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, 1, len(resDecoded.Consumers[0].Consumers))
	require.Equal(t, "partition", resDecoded.Consumers[0].Consumers[0].GetPartition())
}

func TestValidateComputedExportedServices_InvalidName(t *testing.T) {
	res := createComputedExportedServicesResource(t, validComputedExportedServicesWithPartition(), "computed-service")

	err := ValidateComputedExportedServices(res)
	require.Error(t, err)
	expectedError := errors.New("invalid \"name\" field: name can only be \"global\"")
	require.ErrorAs(t, err, &expectedError)
}
