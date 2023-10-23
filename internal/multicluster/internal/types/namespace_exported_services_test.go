// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"testing"
)

func validNamespaceExportedServicesWithPeer(peerName string) *multiclusterv1alpha1.NamespaceExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Peer{
				Peer: peerName,
			},
		},
	}
	return &multiclusterv1alpha1.NamespaceExportedServices{
		Consumers: consumers,
	}
}

func validNamespaceExportedServicesWithPartition(partitionName string) *multiclusterv1alpha1.NamespaceExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Partition{
				Partition: partitionName,
			},
		},
	}
	return &multiclusterv1alpha1.NamespaceExportedServices{
		Consumers: consumers,
	}
}

func validNamespaceExportedServicesWithSamenessGroup(samenessGroupName string) *multiclusterv1alpha1.NamespaceExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_SamenessGroup{
				SamenessGroup: samenessGroupName,
			},
		},
	}
	return &multiclusterv1alpha1.NamespaceExportedServices{
		Consumers: consumers,
	}
}
func TestNamespaceExportedServicesACLs(t *testing.T) {
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
			listOK:  ALLOW,
		},
		"mesh write policy": {
			rules:   `mesh = "write"`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  ALLOW,
		},
	}

	exportedServiceData := &multiclusterv1alpha1.NamespaceExportedServices{}
	res := resourcetest.Resource(multiclusterv1alpha1.NamespaceExportedServicesType, "namespace-exported-services").
		WithData(t, exportedServiceData).
		Build()
	resourcetest.ValidateAndNormalize(t, registry, res)

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
