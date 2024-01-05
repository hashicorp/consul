// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/command/resource/client"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	HeaderConsulToken = "x-consul-token"
)

type ResourceGRPC struct {
	C *client.GRPCClient
}

func (resource *ResourceGRPC) Apply(parsedResource *pbresource.Resource) (*pbresource.Resource, error) {
	token, err := resource.C.Config.GetToken()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(context.Background(), HeaderConsulToken, token)
	}

	defer resource.C.Conn.Close()
	writeRsp, err := resource.C.Client.Write(ctx, &pbresource.WriteRequest{Resource: parsedResource})
	if err != nil {
		return nil, err
	}

	return writeRsp.Resource, err
}
