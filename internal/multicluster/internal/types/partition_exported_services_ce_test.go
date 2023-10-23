// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package types

import (
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPartitionExportedServicesValidations(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		err := ValidatePartitionExportedServices(tc.Resource)
		require.NoError(t, err)
	}

	cases := map[string]testcase{
		"partition exported services with peer": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPeer("peer")).
				Build(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPartitionExportedServicesValidations_Error(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		err := ValidatePartitionExportedServices(tc.Resource)
		require.Error(t, err)
	}

	cases := map[string]testcase{
		"partition exported services with partition": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPartition("default")).
				Build(),
		},
		"partition exported services with sameness_group": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithSamenessGroup("sameness_group")).
				Build(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
