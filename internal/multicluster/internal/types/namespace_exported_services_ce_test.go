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

func TestNamespaceExportedServicesValidations(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		err := MutateNamespaceExportedServices(tc.Resource)
		require.NoError(t, err)

		err = ValidateNamespaceExportedServices(tc.Resource)
		require.NoError(t, err)
	}

	cases := map[string]testcase{
		"namespace exported services with peer": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithPeer("peer")).
				Build(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestNamespaceExportedServicesValidations_Error(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		err := MutateNamespaceExportedServices(tc.Resource)
		require.NoError(t, err)

		err = ValidateNamespaceExportedServices(tc.Resource)
		require.Error(t, err)
	}

	cases := map[string]testcase{
		"namespace exported services with partition": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithPartition("default")).
				Build(),
		},
		"namespace exported services with sameness_group": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithSamenessGroup("sameness_group")).
				Build(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
