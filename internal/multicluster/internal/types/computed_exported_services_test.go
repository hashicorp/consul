// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func createComputedExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage, name string) *pbresource.Resource {
	res := resourcetest.Resource(multiclusterv1alpha1.ComputedExportedServicesType, name).
		WithData(t, data).
		Build()
	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validComputedExportedServicesWithPeer() *multiclusterv1alpha1.ComputedExportedServices {
	consumers := []*multiclusterv1alpha1.ComputedExportedService{
		{
			Consumers: []*multiclusterv1alpha1.ComputedExportedServicesConsumer{
				{
					ConsumerTenancy: &multiclusterv1alpha1.ComputedExportedServicesConsumer_Peer{
						Peer: "peer",
					},
				},
			},
		},
	}
	return &multiclusterv1alpha1.ComputedExportedServices{
		Consumers: consumers,
	}
}

func validComputedExportedServicesWithPartition() *multiclusterv1alpha1.ComputedExportedServices {
	consumers := []*multiclusterv1alpha1.ComputedExportedService{
		{
			Consumers: []*multiclusterv1alpha1.ComputedExportedServicesConsumer{
				{
					ConsumerTenancy: &multiclusterv1alpha1.ComputedExportedServicesConsumer_Partition{
						Partition: "partition",
					},
				},
			},
		},
	}
	return &multiclusterv1alpha1.ComputedExportedServices{
		Consumers: consumers,
	}
}

func TestValidateComputedExportedServices(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		err := ValidateComputedExportedServices(tc.Resource)
		require.NoError(t, err)

		resourcetest.MustDecode[*multiclusterv1alpha1.ComputedExportedServices](t, tc.Resource)
	}

	cases := map[string]testcase{
		"computed exported services with peer": {
			Resource: createComputedExportedServicesResource(t, validComputedExportedServicesWithPeer(), ComputedExportedServicesName),
		},
		"computed exported services with partition": {
			Resource: createComputedExportedServicesResource(t, validComputedExportedServicesWithPartition(), ComputedExportedServicesName),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidateComputedExportedServices_InvalidName(t *testing.T) {
	res := createComputedExportedServicesResource(t, validComputedExportedServicesWithPartition(), "computed-service")

	err := ValidateComputedExportedServices(res)
	require.Error(t, err)
	expectedError := errors.New("invalid \"name\" field: name can only be \"global\"")
	require.ErrorAs(t, err, &expectedError)
}
