// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"testing"
)

func validComputedExportedServicesWithPartition(partitionName string) *pbmulticluster.ComputedExportedServices {
	consumers := []*pbmulticluster.ComputedExportedService{
		{
			Consumers: []*pbmulticluster.ComputedExportedServicesConsumer{
				{
					ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Partition{
						Partition: partitionName,
					},
				},
			},
		},
	}
	return &pbmulticluster.ComputedExportedServices{
		Consumers: consumers,
	}
}

func validComputedExportedServicesWithPeer(peerName string) *pbmulticluster.ComputedExportedServices {
	consumers := []*pbmulticluster.ComputedExportedService{
		{
			Consumers: []*pbmulticluster.ComputedExportedServicesConsumer{
				{
					ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
						Peer: peerName,
					},
				},
			},
		},
	}
	return &pbmulticluster.ComputedExportedServices{
		Consumers: consumers,
	}
}

func TestComputedExportedServicesValidations_InvalidName(t *testing.T) {
	res := resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, "computed-exported-services").
		WithData(t, validComputedExportedServicesWithPeer("peer")).
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

	exportedServiceData := &pbmulticluster.ComputedExportedServices{}
	res := resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
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

func TestComputedExportedServicesValidations(t *testing.T) {
	type testcase struct {
		Resource       *pbresource.Resource
		expectErrorCE  []string
		expectErrorENT []string
	}

	isEnterprise := structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty() == "default"

	run := func(t *testing.T, tc testcase) {
		expectError := tc.expectErrorCE
		if isEnterprise {
			expectError = tc.expectErrorENT
		}
		err := ValidateComputedExportedServices(tc.Resource)
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
		"computed exported services with peer": {
			Resource: resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, ComputedExportedServicesName).
				WithData(t, validComputedExportedServicesWithPeer("peer")).
				Build(),
		},
		"computed exported services with partition": {
			Resource: resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, ComputedExportedServicesName).
				WithData(t, validComputedExportedServicesWithPartition("partition")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can only be set in Enterprise`},
		},
		"computed exported services with peer empty": {
			Resource: resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, ComputedExportedServicesName).
				WithData(t, validComputedExportedServicesWithPeer("")).
				Build(),
			expectErrorCE:  []string{`invalid element at index 0 of list "peer": can not be empty`},
			expectErrorENT: []string{`invalid element at index 0 of list "peer": can not be empty`},
		},
		"computed exported services with partition empty": {
			Resource: resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, ComputedExportedServicesName).
				WithData(t, validComputedExportedServicesWithPartition("")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can not be empty`,
				`invalid element at index 0 of list "partition": can only be set in Enterprise`},
			expectErrorENT: []string{`invalid element at index 0 of list "partition": can not be empty`},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
