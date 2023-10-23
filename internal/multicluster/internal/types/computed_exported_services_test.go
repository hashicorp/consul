// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/stretchr/testify/require"
	"testing"
)

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

func TestComputedExportedServicesValidations_InvalidName(t *testing.T) {
	res := resourcetest.Resource(multiclusterv1alpha1.ComputedExportedServicesType, "computed-exported-services").
		WithData(t, validComputedExportedServicesWithPeer()).
		Build()

	err := ValidateComputedExportedServices(res)
	require.Error(t, err)
	expectedError := errors.New("invalid \"name\" field: name can only be \"global\"")
	require.ErrorAs(t, err, &expectedError)
}

func TestComputedExportedServicesACLs(t *testing.T) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	type testcase struct {
		rules   string
		readOK  string
		writeOK string
		listOK  string
	}

	const (
		DENY    = "deny"
		ALLOW   = "allow"
		DEFAULT = "default"
	)

	exportedServiceData := &multiclusterv1alpha1.ComputedExportedServices{}
	res := resourcetest.Resource(multiclusterv1alpha1.ComputedExportedServicesType, "global").
		WithData(t, exportedServiceData).
		Build()
	resourcetest.ValidateAndNormalize(t, registry, res)

	cases := map[string]testcase{
		"no rules": {
			rules:   ``,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"mesh read policy": {
			rules:   `mesh = "read"`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"mesh write policy": {
			rules:   `mesh = "write"`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  ALLOW,
		},
	}

	for _, tc := range cases {
		aclTestCase := resourcetest.ACLTestCase{
			Rules:   tc.rules,
			Res:     res,
			ReadOK:  tc.readOK,
			WriteOK: tc.writeOK,
			ListOK:  tc.listOK,
		}
		resourcetest.RunACLTestCase(t, aclTestCase, registry)
	}
}
