// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancytest

import (
	"context"
	"github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	resource2 "github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy"
	"testing"

	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto/private/prototest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
	"github.com/stretchr/testify/require"
)

func TestReadNamespace_Success(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	res := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		WithData(t, validNamespace()).
		Write(t, cl)

	readRsp, err := cl.Read(context.Background(), &pbresource.ReadRequest{Id: res.Id})
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, res.Id, readRsp.Resource.Id)
}

func TestReadNamespace_NotFound(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	res := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		WithData(t, validNamespace()).Build()

	_, err := cl.Read(context.Background(), &pbresource.ReadRequest{Id: res.Id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())
}

func TestDeleteNamespace_Success(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	res := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		WithData(t, validNamespace()).Write(t, cl)

	readRsp, err := cl.Read(context.Background(), &pbresource.ReadRequest{Id: res.Id})
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, res.Id, readRsp.Resource.Id)

	_, err = cl.Delete(context.Background(), &pbresource.DeleteRequest{Id: res.Id})
	require.NoError(t, err)

	_, err = cl.Read(context.Background(), &pbresource.ReadRequest{Id: res.Id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound.String(), status.Code(err).String())

}

func TestListNamespace_Success(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	res := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		WithData(t, validNamespace()).Write(t, cl)

	require.NotNil(t, res)
	res = rtest.Resource(pbtenancy.NamespaceType, "ns2").
		WithData(t, validNamespace()).Write(t, cl)

	require.NotNil(t, res)

	listRsp, err := cl.List(context.Background(), &pbresource.ListRequest{Type: pbtenancy.NamespaceType, Tenancy: resource2.DefaultPartitionedTenancy()})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 3)
	names := []string{
		listRsp.Resources[0].Id.Name,
		listRsp.Resources[1].Id.Name,
		listRsp.Resources[2].Id.Name,
	}
	require.Contains(t, names, "default")
	require.Contains(t, names, "ns1")
	require.Contains(t, names, "ns2")
}

func validNamespace() *pbtenancy.Namespace {
	return &pbtenancy.Namespace{
		Description: "ns namespace",
	}
}
