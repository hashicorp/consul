// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/hashicorp/go-rootcerts"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/secrets"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/hashicorp/consul/testing/deployer/util"
)

func (s *Sprawl) dialServerGRPC(cluster *topology.Cluster, node *topology.Node, token string) (*grpc.ClientConn, func(), error) {
	var (
		logger = s.logger.With("cluster", cluster.Name)
	)

	tls := &tls.Config{
		ServerName: fmt.Sprintf("server.%s.consul", cluster.Datacenter),
	}

	rootConfig := &rootcerts.Config{
		CACertificate: []byte(s.secrets.ReadGeneric(cluster.Name, secrets.CAPEM)),
	}
	if err := rootcerts.ConfigureTLS(tls, rootConfig); err != nil {
		return nil, nil, err
	}

	return util.DialExposedGRPCConn(
		context.Background(),
		logger,
		node.ExposedPort(8503),
		token,
		tls,
	)
}
