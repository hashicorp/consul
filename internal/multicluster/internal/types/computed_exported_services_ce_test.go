// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package types

import (
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
)

func validComputedExportedServicesWithPeer() *multiclusterv1alpha1.ComputedExportedServices {
	consumers := []*multiclusterv1alpha1.ComputedExportedService{
		{
			Consumers: []*multiclusterv1alpha1.ComputedExportedServicesConsumer{
				{
					ConsumerTenancy: &multiclusterv1alpha1.ComputedExportedServicesConsumer_Peer{
						Peer: "",
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
						Partition: "default",
					},
				},
			},
		},
	}
	return &multiclusterv1alpha1.ComputedExportedServices{
		Consumers: consumers,
	}
}
