// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancytest

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	resource2 "github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy"

	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto/private/prototest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
	"github.com/stretchr/testify/require"
)

func TestWriteNamespace_Success(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	res := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		WithTenancy(resource2.DefaultPartitionedTenancy()).
		WithData(t, validNamespace()).
		Build()

	writeRsp, err := cl.Write(context.Background(), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, res.Id.Type, writeRsp.Resource.Id.Type)
	prototest.AssertDeepEqual(t, res.Id.Tenancy, writeRsp.Resource.Id.Tenancy)
	prototest.AssertDeepEqual(t, res.Id.Name, writeRsp.Resource.Id.Name)
	prototest.AssertDeepEqual(t, res.Data, writeRsp.Resource.Data)
}

func TestReadNamespace_Success(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	writeResp := rtest.Resource(pbtenancy.NamespaceType, "ns1").
		WithData(t, validNamespace()).
		Write(t, cl)

	cases := []struct {
		name     string
		resource *pbresource.Resource
		errMsg   string
	}{
		{
			name: "read namespace",
			resource: rtest.Resource(pbtenancy.NamespaceType, "ns1").
				WithData(t, validNamespace()).
				Build(),
		},
		{
			name: "tenancy units: empty namespace is allowed",
			resource: rtest.Resource(pbtenancy.NamespaceType, "ns1").
				WithTenancy(&pbresource.Tenancy{
					Namespace: "",
				}).
				WithData(t, validNamespace()).
				Build(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			readRsp, err := cl.Read(context.Background(), &pbresource.ReadRequest{Id: tc.resource.Id})
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, writeResp.Id, readRsp.Resource.Id)
			prototest.AssertDeepEqual(t, writeResp.Data, readRsp.Resource.Data)
		})
	}
}

func TestReadNamespace_NotFound(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	cases := []struct {
		name     string
		resource *pbresource.Resource
		errMsg   string
	}{
		{
			name: "namespace with the given name not present",
			resource: rtest.Resource(pbtenancy.NamespaceType, "ns1").
				WithData(t, validNamespace()).
				Build(),
			errMsg: "resource not found",
		},
		{
			name: "tenancy units: partition not present",
			resource: rtest.Resource(pbtenancy.NamespaceType, "ns1").
				WithTenancy(&pbresource.Tenancy{
					Partition: "partition1",
				}).
				WithData(t, validNamespace()).
				Build(),
			errMsg: "partition not found: partition1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cl.Read(context.Background(), &pbresource.ReadRequest{Id: tc.resource.Id})
			require.Error(t, err)
			require.Equal(t, codes.NotFound.String(), status.Code(err).String())
			require.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestReadNamespace_InvalidArgument(t *testing.T) {
	v2TenancyBridge := tenancy.NewV2TenancyBridge()
	config := resource.Config{TenancyBridge: v2TenancyBridge}
	client := svctest.RunResourceServiceWithConfig(t, config, tenancy.RegisterTypes)
	cl := rtest.NewClient(client)

	cases := []struct {
		name     string
		resource *pbresource.Resource
		errMsg   string
	}{
		{
			name: "tenancy units: namespace not empty",
			resource: rtest.Resource(pbtenancy.NamespaceType, "ns1").
				WithTenancy(&pbresource.Tenancy{
					Partition: "default",
					Namespace: "ns2",
				}).
				WithData(t, validNamespace()).
				Build(),
			errMsg: fmt.Sprintf("partition scoped resource %s.%s.%s cannot have a namespace. got: ns2", pbtenancy.NamespaceType.Group, pbtenancy.NamespaceType.GroupVersion, pbtenancy.NamespaceType.Kind),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cl.Read(context.Background(), &pbresource.ReadRequest{Id: tc.resource.Id})
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
			require.Contains(t, err.Error(), tc.errMsg)
		})
	}
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
