// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/version/versiontest"
)

func inValidExportedServices() *pbmulticluster.ExportedServices {
	return &pbmulticluster.ExportedServices{}
}

func exportedServicesWithPeer(peerName string) *pbmulticluster.ExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: peerName,
			},
		},
	}
	return &pbmulticluster.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func exportedServicesWithPartition(partitionName string) *pbmulticluster.ExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{
				Partition: partitionName,
			},
		},
	}
	return &pbmulticluster.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func exportedServicesWithSamenessGroup(samenessGroupName string) *pbmulticluster.ExportedServices {
	consumers := []*pbmulticluster.ExportedServicesConsumer{
		{
			ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_SamenessGroup{
				SamenessGroup: samenessGroupName,
			},
		},
	}
	return &pbmulticluster.ExportedServices{
		Services:  []string{"api", "frontend", "backend"},
		Consumers: consumers,
	}
}

func TestExportedServicesValidation_NoServices(t *testing.T) {
	res := resourcetest.Resource(pbmulticluster.ExportedServicesType, "exportedservices1").
		WithData(t, inValidExportedServices()).
		Build()

	err := ValidateExportedServices(res)
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
		DENY    = resourcetest.DENY
		ALLOW   = resourcetest.ALLOW
		DEFAULT = resourcetest.DEFAULT
	)

	exportedServiceData := &pbmulticluster.ExportedServices{
		Services: []string{"api", "backend"},
	}
	res := resourcetest.Resource(pbmulticluster.ExportedServicesType, "exps").
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
			listOK:  DEFAULT,
		},
		"all services has write policy": {
			rules:   `service "api" { policy = "write" } service "backend" {policy = "write"}`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
		"only one services has read policy": {
			rules:   `service "api" { policy = "read" }`,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"only one services has write policy": {
			rules:   `service "api" { policy = "write" }`,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
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

func TestExportedServicesValidation(t *testing.T) {
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
		err := ValidateExportedServices(tc.Resource)
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
		"exported services with peer": {
			Resource: resourcetest.Resource(pbmulticluster.ExportedServicesType, "exported-services").
				WithData(t, exportedServicesWithPeer("peer")).
				Build(),
		},
		"exported services with partition": {
			Resource: resourcetest.Resource(pbmulticluster.ExportedServicesType, "exported-services").
				WithData(t, exportedServicesWithPartition("partition")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can only be set in Enterprise`},
		},
		"exported services with sameness_group": {
			Resource: resourcetest.Resource(pbmulticluster.ExportedServicesType, "exported-services").
				WithData(t, exportedServicesWithSamenessGroup("sameness_group")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "sameness_group": can only be set in Enterprise`},
		},
		"exported services with peer empty": {
			Resource: resourcetest.Resource(pbmulticluster.ExportedServicesType, "exported-services").
				WithData(t, exportedServicesWithPeer("")).
				Build(),
			expectErrorCE:  []string{`invalid element at index 0 of list "peer": can not be empty or local`},
			expectErrorENT: []string{`invalid element at index 0 of list "peer": can not be empty or local`},
		},
		"exported services with partition empty": {
			Resource: resourcetest.Resource(pbmulticluster.ExportedServicesType, "exported-services").
				WithData(t, exportedServicesWithPartition("")).
				Build(),
			expectErrorCE: []string{`invalid element at index 0 of list "partition": can not be empty`,
				`invalid element at index 0 of list "partition": can only be set in Enterprise`},
			expectErrorENT: []string{`invalid element at index 0 of list "partition": can not be empty`},
		},
		"exported services with sameness_group empty": {
			Resource: resourcetest.Resource(pbmulticluster.ExportedServicesType, "exported-services").
				WithData(t, exportedServicesWithSamenessGroup("")).
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
