// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

func createExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: multiclusterv1alpha1.ExportedServicesType,
			Name: "exported-services-1",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validExportedServicesWithPeer() *multiclusterv1alpha1.ExportedServices {
	consumers := make([]*multiclusterv1alpha1.ExportedServicesConsumer, 1)
	consumers[0] = new(multiclusterv1alpha1.ExportedServicesConsumer)
	consumers[0].ConsumerTenancy = &multiclusterv1alpha1.ExportedServicesConsumer_Peer{Peer: "peer"}
	return &multiclusterv1alpha1.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func validExportedServicesWithPartition() *multiclusterv1alpha1.ExportedServices {
	consumers := make([]*multiclusterv1alpha1.ExportedServicesConsumer, 1)
	consumers[0] = new(multiclusterv1alpha1.ExportedServicesConsumer)
	consumers[0].ConsumerTenancy = &multiclusterv1alpha1.ExportedServicesConsumer_Partition{Partition: "partition"}
	return &multiclusterv1alpha1.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func validExportedServicesWithSamenessGroup() *multiclusterv1alpha1.ExportedServices {
	consumers := make([]*multiclusterv1alpha1.ExportedServicesConsumer, 1)
	consumers[0] = new(multiclusterv1alpha1.ExportedServicesConsumer)
	consumers[0].ConsumerTenancy = &multiclusterv1alpha1.ExportedServicesConsumer_SamenessGroup{SamenessGroup: "sameness_group"}
	return &multiclusterv1alpha1.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func inValidExportedServices() *multiclusterv1alpha1.ExportedServices {
	return &multiclusterv1alpha1.ExportedServices{}
}

func TestValidateExportedServicesWithPeer_Ok(t *testing.T) {
	res := createExportedServicesResource(t, validExportedServicesWithPeer())

	err := ValidateExportedServices(res)
	require.NoError(t, err)

	var resDecoded multiclusterv1alpha1.ExportedServices
	err = res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, "peer", resDecoded.Consumers[0].GetPeer())
}

func TestValidateExportedServicesWithPartition_Ok(t *testing.T) {
	res := createExportedServicesResource(t, validExportedServicesWithPartition())

	err := ValidateExportedServices(res)
	require.NoError(t, err)

	var resDecoded multiclusterv1alpha1.ExportedServices
	err = res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, "partition", resDecoded.Consumers[0].GetPartition())
}

func TestValidateExportedServicesWithSamenessGroup_Ok(t *testing.T) {
	res := createExportedServicesResource(t, validExportedServicesWithSamenessGroup())

	err := ValidateExportedServices(res)
	require.NoError(t, err)

	var resDecoded multiclusterv1alpha1.ExportedServices
	err = res.Data.UnmarshalTo(&resDecoded)
	require.NoError(t, err)
	require.Equal(t, 1, len(resDecoded.Consumers))
	require.Equal(t, "sameness_group", resDecoded.Consumers[0].GetSamenessGroup())
}

func TestValidateExportedServices_NoServices(t *testing.T) {
	res := createExportedServicesResource(t, inValidExportedServices())

	err := ValidateExportedServices(res)
	require.Error(t, err)
	expectedError := errors.New("invalid \"services\" field: at least one service must be set")
	require.ErrorAs(t, err, &expectedError)
}
