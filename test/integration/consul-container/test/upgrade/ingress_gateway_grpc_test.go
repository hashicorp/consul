// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package upgrade

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

func TestIngressGateway_GRPC_UpgradeToTarget_fromLatest(t *testing.T) {
	t.Parallel()
	cluster, _, client := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.GetLatestImageName(),
			ConsulVersion:   utils.LatestVersion,
		},
		ApplyDefaultProxySettings: true,
	})

	require.NoError(t, cluster.ConfigEntryWrite(&api.ServiceConfigEntry{
		Name:     libservice.StaticServerServiceName,
		Kind:     api.ServiceDefaults,
		Protocol: "grpc",
	}))

	const (
		nameIG = "ingress-gateway"
	)

	const nameS1 = libservice.StaticServerServiceName

	igw, err := libservice.NewGatewayService(
		context.Background(),
		libservice.GatewayConfig{
			Name: nameIG,
			Kind: "ingress",
		},
		cluster.Servers()[0],
	)
	require.NoError(t, err)

	// these must be one of the externally-mapped ports from
	// https://github.com/hashicorp/consul/blob/c5e729e86576771c4c22c6da1e57aaa377319323/test/integration/consul-container/libs/cluster/container.go#L521-L525
	const portS1DirectNoTLS = 8080
	require.NoError(t, cluster.ConfigEntryWrite(&api.IngressGatewayConfigEntry{
		Kind: api.IngressGateway,
		Name: nameIG,
		Listeners: []api.IngressListener{
			{
				Port:     portS1DirectNoTLS,
				Protocol: "grpc",
				Services: []api.IngressService{
					{
						Name:  libservice.StaticServerServiceName,
						Hosts: []string{"*"},
					},
				},
			},
		},
	}))

	// register static-server service
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(
		cluster.Clients()[0],
		&libservice.ServiceOpts{
			Name:         nameS1,
			ID:           nameS1,
			HTTPPort:     8080,
			GRPCPort:     8079,
			RegisterGRPC: true,
		},
	)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, nameS1, nil)

	// Register an static-client service
	serverNodes := cluster.Servers()
	require.NoError(t, err)
	require.True(t, len(serverNodes) > 0)
	staticClientSvcSidecar, err := libservice.CreateAndRegisterStaticClientSidecar(serverNodes[0], "", true, false, nil)
	require.NoError(t, err)

	tests := func(t *testing.T) {
		t.Run("grpc directly", func(t *testing.T) {
			_, p := staticClientSvcSidecar.GetAddr()
			libassert.GRPCPing(t, fmt.Sprintf("localhost:%d", p))
		})
		t.Run("grpc via igw", func(t *testing.T) {
			pm, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", portS1DirectNoTLS)),
			)
			libassert.GRPCPing(t, fmt.Sprintf("localhost:%d", pm.Int()))
		})
	}

	t.Run("pre-upgrade", func(t *testing.T) {
		tests(t)
	})

	if t.Failed() {
		t.Fatal("failing fast: failed assertions pre-upgrade")
	}

	t.Logf("Upgrade to version %s", utils.TargetVersion)
	err = cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion)
	require.NoError(t, err)
	require.NoError(t, igw.Restart())

	t.Run("post-upgrade", func(t *testing.T) {
		tests(t)
	})
}
