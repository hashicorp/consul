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

func createExportedServicesResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := resourcetest.Resource(multiclusterv1alpha1.ExportedServicesType, "exported-services-1").
		WithData(t, data).
		Build()

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validExportedServicesWithPeer() *multiclusterv1alpha1.ExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Peer{
				Peer: "peer",
			},
		},
	}
	return &multiclusterv1alpha1.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func validExportedServicesWithPartition() *multiclusterv1alpha1.ExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Partition{
				Partition: "partition",
			},
		},
	}
	return &multiclusterv1alpha1.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func validExportedServicesWithSamenessGroup() *multiclusterv1alpha1.ExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_SamenessGroup{
				SamenessGroup: "sameness_group",
			},
		},
	}
	return &multiclusterv1alpha1.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func inValidExportedServices() *multiclusterv1alpha1.ExportedServices {
	return &multiclusterv1alpha1.ExportedServices{}
}

func TestValidateExportedServices(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		err := ValidateExportedServices(tc.Resource)
		require.NoError(t, err)

		resourcetest.MustDecode[*multiclusterv1alpha1.ExportedServices](t, tc.Resource)
	}

	cases := map[string]testcase{
		"exported services with peer": {
			Resource: createExportedServicesResource(t, validExportedServicesWithPeer()),
		},
		"exported services with partition": {
			Resource: createExportedServicesResource(t, validExportedServicesWithPartition()),
		},
		"exported services with sameness_group": {
			Resource: createExportedServicesResource(t, validExportedServicesWithSamenessGroup()),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidateExportedServices_NoServices(t *testing.T) {
	res := createExportedServicesResource(t, inValidExportedServices())

	err := ValidateExportedServices(res)
	require.Error(t, err)
	expectedError := errors.New("invalid \"services\" field: at least one service must be set")
	require.ErrorAs(t, err, &expectedError)
}
