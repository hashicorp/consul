// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

func createNamespaceResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbtenancy.NamespaceType,
			Tenancy: resource.DefaultPartitionedTenancy(),
			Name:    "ns1234",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateNamespace_Ok(t *testing.T) {
	res := createNamespaceResource(t, validNamespace())

	err := ValidateNamespace(res)
	require.NoError(t, err)
}

func TestValidateNamespace_defaultNamespace(t *testing.T) {
	res := createNamespaceResource(t, validNamespace())
	res.Id.Name = resource.DefaultNamespaceName

	err := ValidateNamespace(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &errInvalidName)
}

func TestValidateNamespace_defaultNamespaceNonDefaultPartition(t *testing.T) {
	res := createNamespaceResource(t, validNamespace())
	res.Id.Name = resource.DefaultNamespaceName
	res.Id.Tenancy.Partition = "foo"

	err := ValidateNamespace(res)
	require.NoError(t, err)
}

func TestValidateNamespace_InvalidName(t *testing.T) {
	res := createNamespaceResource(t, validNamespace())
	res.Id.Name = "-invalid"

	err := ValidateNamespace(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &errInvalidName)
}

func TestValidateNamespace_InvalidOwner(t *testing.T) {
	res := createNamespaceResource(t, validNamespace())
	res.Owner = &pbresource.ID{}
	err := ValidateNamespace(res)

	require.Error(t, err)
	require.ErrorAs(t, err, &errOwnerNonEmpty)
}

func TestValidateNamespace_ParseError(t *testing.T) {
	// Any type other than the Namespace type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createNamespaceResource(t, data)

	err := ValidateNamespace(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name          string
		namespaceName string
		err           string
	}{
		{"system", "System", "namespace \"System\" is reserved for future internal use"},
		{"invalid", "-inval", "namespace name \"-inval\" is not a valid DNS hostname"},
		{"valid", "ns1", ""},
		{"space prefix", " foo", "namespace name \" foo\" is not a valid DNS hostname"},
		{"space suffix", "bar ", "namespace name \"bar \" is not a valid DNS hostname"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := anypb.New(&pbtenancy.Namespace{})
			require.NoError(t, err)
			res := &pbresource.Resource{Id: &pbresource.ID{Name: tt.namespaceName}, Data: a}
			err = ValidateNamespace(res)
			if tt.err == "" {
				require.NoError(t, err)
			} else {
				require.Equal(t, err.Error(), tt.err)
			}

		})
	}
}

func validNamespace() *pbtenancy.Namespace {
	return &pbtenancy.Namespace{
		Description: "ns namespace",
	}
}
