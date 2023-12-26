// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"

	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/testrpc"
)

func TestConfigEntryBackend_RejectsPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.GRPCTLSPort = freeport.GetOne(t)
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// make a grpc client to dial s1 directly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, s1.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		//nolint:staticcheck
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	configEntryClient := pbconfigentry.NewConfigEntryServiceClient(conn)

	req := pbconfigentry.GetResolvedExportedServicesRequest{
		Partition: "test",
	}
	_, err = configEntryClient.GetResolvedExportedServices(ctx, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Partitions are a Consul Enterprise feature")
}

func TestConfigEntryBackend_IgnoresDefaultPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.GRPCTLSPort = freeport.GetOne(t)
	})

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// make a grpc client to dial s1 directly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, s1.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		//nolint:staticcheck
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	configEntryClient := pbconfigentry.NewConfigEntryServiceClient(conn)

	req := pbconfigentry.GetResolvedExportedServicesRequest{
		Partition: "DeFaUlT",
	}
	_, err = configEntryClient.GetResolvedExportedServices(ctx, &req)
	require.NoError(t, err)
}
