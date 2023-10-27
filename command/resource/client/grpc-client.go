// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type GRPCClient struct {
	Client pbresource.ResourceServiceClient
	Config *GRPCConfig
	Conn   *grpc.ClientConn
}

func NewGRPCClient(config *GRPCConfig) (*GRPCClient, error) {
	conn, err := dial(config)
	if err != nil {
		return nil, fmt.Errorf("**** error dialing grpc: %+v", err)
	}
	return &GRPCClient{
		Client: pbresource.NewResourceServiceClient(conn),
		Config: config,
		Conn:   conn,
	}, nil
}

func dial(c *GRPCConfig) (*grpc.ClientConn, error) {
	dialCtx := context.Background()
	// TODO: decide if we use TLS mode based on the config
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	return grpc.DialContext(dialCtx, c.Address, dialOpts...)
}
