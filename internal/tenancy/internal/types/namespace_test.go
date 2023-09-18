// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	tenancyv1alpha1 "github.com/hashicorp/consul/proto-public/pbtenancy/v1alpha1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func createNamespaceResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: NamespaceV1Alpha1Type,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "",
				PeerName:  "local",
			},
			Name: "ns-1234",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validNamespace() *tenancyv1alpha1.Namespace {
	return &tenancyv1alpha1.Namespace{
		Description: "description from user",
	}
}

func TestValidateNamespace_Ok(t *testing.T) {
	res := createNamespaceResource(t, validNamespace())

	err := ValidateNamespace(res)
	require.NoError(t, err)
}

func TestValidateNamespace_defaultNamespace(t *testing.T) {
	res := createNamespaceResource(t, validNamespace())
	res.Id.Name = resource.DefaultNamespacedTenancy().Namespace

	err := ValidateNamespace(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &errInvalidName)
}

func TestValidateNamespace_ParseError(t *testing.T) {
	// Any type other than the Workload type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createNamespaceResource(t, data)

	err := ValidateNamespace(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}
