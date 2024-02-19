// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/version/versiontest"
)

func validNamespaceExportedServicesWithPeer(peerName string) *pbmulticluster.NamespaceExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: peerName,
			},
		},
	}
	return &pbmulticluster.NamespaceExportedServices{
		Consumers: consumers,
	}
}

func validNamespaceExportedServicesWithPartition(partitionName string) *pbmulticluster.NamespaceExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{
				Partition: partitionName,
			},
		},
	}
	return &pbmulticluster.NamespaceExportedServices{
		Consumers: consumers,
	}
}

func validNamespaceExportedServicesWithSamenessGroup(samenessGroupName string) *pbmulticluster.NamespaceExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_SamenessGroup{
				SamenessGroup: samenessGroupName,
			},
		},
	}
	return &pbmulticluster.NamespaceExportedServices{
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
		DENY    = resourcetest.DENY
		ALLOW   = resourcetest.ALLOW
		DEFAULT = resourcetest.DEFAULT
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
			listOK:  DEFAULT,
		},
		"mesh write policy": {
			rules:   `mesh = "write"`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
	}

	exportedServiceData := &pbmulticluster.NamespaceExportedServices{}
	res := resourcetest.Resource(pbmulticluster.NamespaceExportedServicesType, "namespace-exported-services").
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

func TestNamespaceExportedServicesValidations(t *testing.T) {
	type testcase struct {
		Resource       *pbresource.Resource
		expectErrorCE  []string
		expectErrorENT []string
	}

	isEnterprise := versiontest.IsEnterprise()

	run := func(t *testing.T, tc testcase) {
		expectError := tc.expectErrorCE
		if isEnterprise {
			expectError = tc.expectErrorENT
		}
		err := ValidateNamespaceExportedServices(tc.Resource)
		if len(expectError) == 0 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			for _, er := range expectError {
				require.ErrorContains(t, err, er)
			}
		}
	}

	cases := map[string]testcase{
		"namespace exported services with peer": {
			Resource: resourcetest.Resource(pbmulticluster.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithPeer("peer")).
				Build(),
		},
		"namespace exported services with partition": {
			Resource: resourcetest.Resource(pbmulticluster.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithPartition("partition")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can only be set in Enterprise`},
		},
		"namespace exported services with sameness_group": {
			Resource: resourcetest.Resource(pbmulticluster.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithSamenessGroup("sameness_group")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "sameness_group": can only be set in Enterprise`},
		},
		"namespace exported services with peer empty": {
			Resource: resourcetest.Resource(pbmulticluster.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithPeer("")).
				Build(),
			expectErrorCE:  []string{`invalid element at index 0 of list "peer": can not be empty or local`},
			expectErrorENT: []string{`invalid element at index 0 of list "peer": can not be empty or local`},
		},
		"namespace exported services with partition empty": {
			Resource: resourcetest.Resource(pbmulticluster.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithPartition("")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can not be empty`,
				`invalid element at index 0 of list "partition": can only be set in Enterprise`},
			expectErrorENT: []string{`invalid element at index 0 of list "partition": can not be empty`},
		},
		"namespace exported services with sameness_group empty": {
			Resource: resourcetest.Resource(pbmulticluster.NamespaceExportedServicesType, "namespace-exported-services").
				WithData(t, validNamespaceExportedServicesWithSamenessGroup("")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "sameness_group": can not be empty`,
				`invalid element at index 0 of list "sameness_group": can only be set in Enterprise`},
			expectErrorENT: []string{`invalid element at index 0 of list "sameness_group": can not be empty`},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
