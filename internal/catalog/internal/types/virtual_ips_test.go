// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func createVirtualIPsResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: VirtualIPsType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
				PeerName:  "local",
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
