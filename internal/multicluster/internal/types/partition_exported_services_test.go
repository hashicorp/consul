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

func validPartitionExportedServicesWithPeer(peerName string) *pbmulticluster.PartitionExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: peerName,
			},
		},
	}
	return &pbmulticluster.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithPartition(partitionName string) *pbmulticluster.PartitionExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{
				Partition: partitionName,
			},
		},
	}
	return &pbmulticluster.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithSamenessGroup(samenessGroupName string) *pbmulticluster.PartitionExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_SamenessGroup{
				SamenessGroup: samenessGroupName,
			},
		},
	}
	return &pbmulticluster.PartitionExportedServices{
		Consumers: consumers,
	}
}

func TestPartitionExportedServicesACLs(t *testing.T) {
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

	exportedServiceData := &pbmulticluster.PartitionExportedServices{}
	res := resourcetest.Resource(pbmulticluster.PartitionExportedServicesType, "partition-exported-services").
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

func TestPartitionExportedServicesValidations(t *testing.T) {
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
		err := ValidatePartitionExportedServices(tc.Resource)
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
		"partition exported services with peer": {
			Resource: resourcetest.Resource(pbmulticluster.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPeer("peer")).
				Build(),
		},
		"partition exported services with partition": {
			Resource: resourcetest.Resource(pbmulticluster.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPartition("partition")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can only be set in Enterprise`},
		},
		"partition exported services with sameness_group": {
			Resource: resourcetest.Resource(pbmulticluster.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithSamenessGroup("sameness_group")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "sameness_group": can only be set in Enterprise`},
		},
		"partition exported services with peer empty": {
			Resource: resourcetest.Resource(pbmulticluster.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPeer("")).
				Build(),
			expectErrorCE:  []string{`invalid element at index 0 of list "peer": can not be empty or local`},
			expectErrorENT: []string{`invalid element at index 0 of list "peer": can not be empty or local`},
		},
		"partition exported services with partition empty": {
			Resource: resourcetest.Resource(pbmulticluster.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPartition("")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can not be empty`,
				`invalid element at index 0 of list "partition": can only be set in Enterprise`},
			expectErrorENT: []string{`invalid element at index 0 of list "partition": can not be empty`},
		},
		"partition exported services with sameness_group empty": {
			Resource: resourcetest.Resource(pbmulticluster.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithSamenessGroup("")).
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
