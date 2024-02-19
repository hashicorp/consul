// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func createVirtualIPsResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: pbcatalog.VirtualIPsType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
			},
			Name: "test-vip",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateVirtualIPs_Ok(t *testing.T) {
	data := &pbcatalog.VirtualIPs{
		Ips: []*pbcatalog.IP{
			{
				Address:   "::ffff:192.0.2.128",
				Generated: true,
			},
			{
				Address:   "10.0.1.42",
				Generated: false,
			},
		},
	}

	res := createVirtualIPsResource(t, data)

	err := ValidateVirtualIPs(res)
	require.NoError(t, err)
}

func TestValidateVirtualIPs_ParseError(t *testing.T) {
	// Any type other than the VirtualIPs type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createVirtualIPsResource(t, data)

	err := ValidateVirtualIPs(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateVirtualIPs_InvalidIP(t *testing.T) {
	data := &pbcatalog.VirtualIPs{
		Ips: []*pbcatalog.IP{
			{
				Address: "foo",
			},
		},
	}

	res := createVirtualIPsResource(t, data)

	err := ValidateVirtualIPs(res)
	require.Error(t, err)
	require.ErrorIs(t, err, errNotIPAddress)
}

func TestVirtualIPsACLs(t *testing.T) {
	registry := resource.NewRegistry()
	RegisterVirtualIPs(registry)

	service := rtest.Resource(pbcatalog.ServiceType, "test").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	virtualIPsData := &pbcatalog.VirtualIPs{}
	cases := map[string]rtest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    virtualIPsData,
			Owner:   service,
			Typ:     pbcatalog.VirtualIPsType,
			ReadOK:  rtest.DENY,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test read": {
			Rules:   `service "test" { policy = "read" }`,
			Data:    virtualIPsData,
			Owner:   service,
			Typ:     pbcatalog.VirtualIPsType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test write": {
			Rules:   `service "test" { policy = "write" }`,
			Data:    virtualIPsData,
			Owner:   service,
			Typ:     pbcatalog.VirtualIPsType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.ALLOW,
			ListOK:  rtest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rtest.RunACLTestCase(t, tc, registry)
		})
	}
}
