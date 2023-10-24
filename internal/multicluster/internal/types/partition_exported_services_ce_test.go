// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package types

import (
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
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
		Resource    *pbresource.Resource
		expectError string
	}
	run := func(t *testing.T, tc testcase) {
		err := ValidatePartitionExportedServices(tc.Resource)
		require.Error(t, err)
		testutil.RequireErrorContains(t, err, tc.expectError)
	}

	cases := map[string]testcase{
		"partition exported services with partition": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPartition("default")).
				Build(),
			expectError: `invalid element at index 0 of list "partition": can only be set in Enterprise`,
		},
		"partition exported services with sameness_group": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithSamenessGroup("sameness_group")).
				Build(),
			expectError: `invalid element at index 0 of list "sameness_group": can only be set in Enterprise`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
