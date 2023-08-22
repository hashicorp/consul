// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type ServiceEndpointsData struct {
	Resource  *pbresource.Resource
	Endpoints *pbcatalog.ServiceEndpoints
}

type ProxyStateTemplateData struct {
	Resource *pbresource.Resource
	Template *pbmesh.ProxyStateTemplate
}

// getServiceEndpoints will return a non-nil &ServiceEndpointsData unless there is an error.
func getServiceEndpoints(ctx context.Context, rt controller.Runtime, id *pbresource.ID) (*ServiceEndpointsData, error) {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: id})
	if err != nil {
		return nil, err
	}

	var se pbcatalog.ServiceEndpoints
	err = rsp.Resource.Data.UnmarshalTo(&se)
	if err != nil {
		return nil, resource.NewErrDataParse(&se, err)
	}

	return &ServiceEndpointsData{Resource: rsp.Resource, Endpoints: &se}, nil
}

func getProxyStateTemplate(ctx context.Context, rt controller.Runtime, id *pbresource.ID) (*ProxyStateTemplateData, error) {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: id})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	var pst pbmesh.ProxyStateTemplate
	err = rsp.Resource.Data.UnmarshalTo(&pst)
	if err != nil {
		return nil, resource.NewErrDataParse(&pst, err)
	}

	return &ProxyStateTemplateData{Resource: rsp.Resource, Template: &pst}, nil
}
