// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func createNodeResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: NodeType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
				PeerName:  "local",
			},
			Name: "test-node",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateNode_Ok(t *testing.T) {
	data := &pbcatalog.Node{
		Addresses: []*pbcatalog.NodeAddress{
			{
				Host: "198.18.0.1",
			},
			{
				Host: "fe80::316a:ed5b:f62c:7321",
			},
			{
				Host:     "foo.node.example.com",
				External: true,
			},
		},
	}

	res := createNodeResource(t, data)

	err := ValidateNode(res)
	require.NoError(t, err)
}

func TestValidateNode_ParseError(t *testing.T) {
	// Any type other than the Node type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createNodeResource(t, data)

	err := ValidateNode(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateNode_EmptyAddresses(t *testing.T) {
	data := &pbcatalog.Node{}

	res := createNodeResource(t, data)

	err := ValidateNode(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "addresses",
		Wrapped: resource.ErrEmpty,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateNode_InvalidAddress(t *testing.T) {
	data := &pbcatalog.Node{
		Addresses: []*pbcatalog.NodeAddress{
			{
				Host: "unix:///node.sock",
			},
		},
	}

	res := createNodeResource(t, data)

	err := ValidateNode(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "host",
		Wrapped: errInvalidNodeHostFormat{Host: "unix:///node.sock"},
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateNode_AddressMissingHost(t *testing.T) {
	data := &pbcatalog.Node{
		Addresses: []*pbcatalog.NodeAddress{
			{},
		},
	}

	res := createNodeResource(t, data)

	err := ValidateNode(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "host",
		Wrapped: resource.ErrMissing,
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}
