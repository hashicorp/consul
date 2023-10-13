// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"testing"
)

func validPartitionExportedServicesWithPeer() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Peer{
				Peer: "",
			},
		},
	}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithPartition() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_Partition{
				Partition: "default",
			},
		},
	}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func validPartitionExportedServicesWithSamenessGroup() *multiclusterv1alpha1.PartitionExportedServices {
	consumers := []*multiclusterv1alpha1.ExportedServicesConsumer{
		{
			ConsumerTenancy: &multiclusterv1alpha1.ExportedServicesConsumer_SamenessGroup{
				SamenessGroup: "",
			},
		},
	}
	return &multiclusterv1alpha1.PartitionExportedServices{
		Consumers: consumers,
	}
}

func TestPartitionExportedServicesValidations(t *testing.T) {
	type testcase struct {
		Resource *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		err := ValidatePartitionExportedServices(tc.Resource)
		require.NoError(t, err)

		resourcetest.MustDecode[*multiclusterv1alpha1.PartitionExportedServices](t, tc.Resource)
	}

	cases := map[string]testcase{
		"partition exported services with peer": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPeer()).
				Build(),
		},
		"partition exported services with partition": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithPartition()).
				Build(),
		},
		"partition exported services with sameness_group": {
			Resource: resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
				WithData(t, validPartitionExportedServicesWithSamenessGroup()).
				Build(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPartitionExportedServicesACLs(t *testing.T) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	type testcase struct {
		rules   string
		check   func(t *testing.T, authz acl.Authorizer, res *pbresource.Resource)
		readOK  string
		writeOK string
		listOK  string
	}

	const (
		DENY    = "deny"
		ALLOW   = "allow"
		DEFAULT = "default"
	)

	checkF := func(t *testing.T, expect string, got error) {
		switch expect {
		case ALLOW:
			if acl.IsErrPermissionDenied(got) {
				t.Fatal("should be allowed")
			}
		case DENY:
			if !acl.IsErrPermissionDenied(got) {
				t.Fatal("should be denied")
			}
		case DEFAULT:
			require.Nil(t, got, "expected fallthrough decision")
		default:
			t.Fatalf("unexpected expectation: %q", expect)
		}
	}

	reg, ok := registry.Resolve(multiclusterv1alpha1.PartitionExportedServicesType)
	require.True(t, ok)

	run := func(t *testing.T, tc testcase) {
		exportedServiceData := &multiclusterv1alpha1.PartitionExportedServices{}
		res := resourcetest.Resource(multiclusterv1alpha1.PartitionExportedServicesType, "partition-exported-services").
			WithData(t, exportedServiceData).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res)

		config := acl.Config{
			WildcardName: structs.WildcardSpecifier,
		}
		authz, err := acl.NewAuthorizerFromRules(tc.rules, &config, nil)
		require.NoError(t, err)
		authz = acl.NewChainedAuthorizer([]acl.Authorizer{authz, acl.DenyAll()})

		t.Run("read", func(t *testing.T) {
			err := reg.ACLs.Read(authz, &acl.AuthorizerContext{}, res.Id, res)
			checkF(t, tc.readOK, err)
		})
		t.Run("write", func(t *testing.T) {
			err := reg.ACLs.Write(authz, &acl.AuthorizerContext{}, res)
			checkF(t, tc.writeOK, err)
		})
		t.Run("list", func(t *testing.T) {
			err := reg.ACLs.List(authz, &acl.AuthorizerContext{})
			checkF(t, tc.listOK, err)
		})
	}

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

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
