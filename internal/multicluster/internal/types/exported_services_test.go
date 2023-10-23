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

func inValidExportedServices() *multiclusterv1alpha1.ExportedServices {
	return &multiclusterv1alpha1.ExportedServices{}
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

func TestExportedServicesValidation_NoServices(t *testing.T) {
	res := resourcetest.Resource(multiclusterv1alpha1.ExportedServicesType, "exportedservices1").
		WithData(t, inValidExportedServices()).
		Build()

	err := MutateExportedServices(res)
	require.NoError(t, err)

	err = ValidateExportedServices(res)
	require.Error(t, err)
	expectedError := errors.New("invalid \"services\" field: at least one service must be set")
	require.ErrorAs(t, err, &expectedError)
}

func TestExportedServicesACLs(t *testing.T) {
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

	exportedServiceData := &multiclusterv1alpha1.ExportedServices{
		Services: []string{"api", "backend"},
	}
	res := resourcetest.Resource(multiclusterv1alpha1.ExportedServicesType, "exps").
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
		"all services has read policy": {
			rules:   `service "api" { policy = "read" } service "backend" {policy = "read"}`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  ALLOW,
		},
		"all services has write policy": {
			rules:   `service "api" { policy = "write" } service "backend" {policy = "write"}`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  ALLOW,
		},
		"only one services has read policy": {
			rules:   `service "api" { policy = "read" }`,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  ALLOW,
		},
		"only one services has write policy": {
			rules:   `service "api" { policy = "write" }`,
			readOK:  DENY,
			writeOK: DENY,
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
