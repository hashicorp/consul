// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
)

const (
	HeaderConsulToken = "x-consul-token"
)

type GRPCClient struct {
	Client pbresource.ResourceServiceClient
	Config *GRPCConfig
	Conn   *grpc.ClientConn
}

func NewGRPCClient(config *GRPCConfig) (*GRPCClient, error) {
	conn, err := dial(config)
	if err != nil {
		return nil, fmt.Errorf("error dialing grpc: %+v", err)
	}
	return &GRPCClient{
		Client: pbresource.NewResourceServiceClient(conn),
		Config: config,
		Conn:   conn,
	}, nil
}

func (client *GRPCClient) Apply(parsedResource *pbresource.Resource) (*pbresource.Resource, error) {
	token, err := client.Config.GetToken()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, HeaderConsulToken, token)
	}

	defer client.Conn.Close()
	writeRsp, err := client.Client.Write(ctx, &pbresource.WriteRequest{Resource: parsedResource})
	if err != nil {
		return nil, fmt.Errorf("error writing resource: %+v", err)
	}

	return writeRsp.Resource, err
}

func (client *GRPCClient) Read(resourceType *pbresource.Type, resourceTenancy *pbresource.Tenancy, resourceName string, stale bool) (*pbresource.Resource, error) {
	token, err := client.Config.GetToken()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if !stale {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-consul-consistency-mode", "consistent")
	}
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, HeaderConsulToken, token)
	}

	defer client.Conn.Close()
	readRsp, err := client.Client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Type:    resourceType,
			Tenancy: resourceTenancy,
			Name:    resourceName,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("error reading resource: %+v", err)
	}

	return readRsp.Resource, err
}

func (client *GRPCClient) List(resourceType *pbresource.Type, resourceTenancy *pbresource.Tenancy, prefix string, stale bool) ([]*pbresource.Resource, error) {
	token, err := client.Config.GetToken()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if !stale {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-consul-consistency-mode", "consistent")
	}
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(context.Background(), HeaderConsulToken, token)
	}

	defer client.Conn.Close()
	listRsp, err := client.Client.List(ctx, &pbresource.ListRequest{
		Type:       resourceType,
		Tenancy:    resourceTenancy,
		NamePrefix: prefix,
	})

	if err != nil {
		return nil, fmt.Errorf("error listing resource: %+v", err)
	}

	return listRsp.Resources, err
}

func (client *GRPCClient) Delete(resourceType *pbresource.Type, resourceTenancy *pbresource.Tenancy, resourceName string) error {
	token, err := client.Config.GetToken()
	if err != nil {
		return err
	}
	ctx := context.Background()
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(context.Background(), HeaderConsulToken, token)
	}

	defer client.Conn.Close()
	_, err = client.Client.Delete(ctx, &pbresource.DeleteRequest{
		Id: &pbresource.ID{
			Type:    resourceType,
			Tenancy: resourceTenancy,
			Name:    resourceName,
		},
	})

	if err != nil {
		return fmt.Errorf("error deleting resource: %+v", err)
	}

	return nil
}

func dial(c *GRPCConfig) (*grpc.ClientConn, error) {
	err := checkCertificates(c)
	if err != nil {
		return nil, err
	}
	var dialOpts []grpc.DialOption
	if c.GRPCTLS {
		tlsConfig, err := SetupTLSConfig(c)
		if err != nil {
			return nil, fmt.Errorf("failed to setup tls config when tried to establish grpc call: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return grpc.Dial(c.Address, dialOpts...)
}

func checkCertificates(c *GRPCConfig) error {
	if c.GRPCTLS {
		certFileEmpty := c.CertFile == ""
		keyFileEmpty := c.KeyFile == ""

		// both files need to be empty or both files need to be provided
		if certFileEmpty != keyFileEmpty {
			return fmt.Errorf("you have to provide client certificate file and key file at the same time " +
				"if you intend to communicate in TLS/SSL mode")
		}
	}
	return nil
}
