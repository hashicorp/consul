// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func createPartitionExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
		WithData(t, data).
		Build()

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validPartitionExportedServicesWithPeer() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Peer{
				Peer: "peer",
			},
		},
	}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithPartition() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Partition{
				Partition: "partition",
			},
		},
	}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithSamenessGroup() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_SamenessGroup{
				SamenessGroup: "sameness_group",
			},
		},
	}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func TestPartitionExportedServices(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		resourcetest.MustDecode[*multiclusterv1alpha1.PartitionExportedServices](t, tc.Resource)
	}

	cases := map[string]testcase{
		"partition exported services with peer": {
			Resource: createPartitionExportedServicesResource(t, validPartitionExportedServicesWithPeer()),
		},
		"partition exported services with partition": {
			Resource: createPartitionExportedServicesResource(t, validPartitionExportedServicesWithPartition()),
		},
		"partition exported services with sameness_group": {
			Resource: createPartitionExportedServicesResource(t, validPartitionExportedServicesWithSamenessGroup()),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
