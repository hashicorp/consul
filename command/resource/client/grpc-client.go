// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
		keyFileEmpty := c.CertFile == ""

		// both files need to be empty or both files need to be provided
		if certFileEmpty != keyFileEmpty {
			return fmt.Errorf("you have to provide client certificate file and key file at the same time " +
				"if you intend to communicate in TLS/SSL mode")
		}
	}
	return nil
}
