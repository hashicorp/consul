// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package upgrade

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const sdsServerPort = 1234

// This upgrade test tests Ingress Gateway functionality when using an external
// SDS server for certs, as described in https://developer.hashicorp.com/consul/docs/connect/gateways/ingress-gateway#custom-tls-certificates-via-secret-discovery-service-sds
// It:
//  1. starts a consul cluster
//  2. builds and starts a test SDS server from .../test-sds-server
//  3. configures an ingress gateway pointed at this SDS server
//  4. does HTTPS calls against the gateway and checks that the certs returned
//     are from the SDS server as expected
func TestIngressGateway_SDS_UpgradeToTarget_fromLatest(t *testing.T) {
	t.Parallel()

	cluster, _, client := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 1,
		NumClients: 2,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:      "dc1",
			ConsulImageName: utils.GetLatestImageName(),
			ConsulVersion:   utils.LatestVersion,
		},
		ApplyDefaultProxySettings: true,
	})

	sdsServerContainerName, rootPEM := createSDSServer(t, cluster)

	require.NoError(t, cluster.ConfigEntryWrite(&api.ServiceConfigEntry{
		Name:     libservice.StaticServerServiceName,
		Kind:     api.ServiceDefaults,
		Protocol: "http",
	}))
	require.NoError(t, cluster.ConfigEntryWrite(&api.ServiceConfigEntry{
		Name:     libservice.StaticServer2ServiceName,
		Kind:     api.ServiceDefaults,
		Protocol: "http",
	}))

	const (
		nameIG = "ingress-gateway"
	)

	const nameS1 = libservice.StaticServerServiceName
	const nameS2 = libservice.StaticServer2ServiceName

	// this must be one of the externally-mapped ports from
	// https://github.com/hashicorp/consul/blob/c5e729e86576771c4c22c6da1e57aaa377319323/test/integration/consul-container/libs/cluster/container.go#L521-L525
	const (
		portWildcard   = 8080
		portOther      = 9999
		nameSDSCluster = "sds-cluster"
		// these are in our pre-created certs in .../test-sds-server
		hostnameWWW          = "www.example.com"
		hostnameFoo          = "foo.example.com"
		certResourceWildcard = "wildcard.ingress.consul"
	)
	require.NoError(t, cluster.ConfigEntryWrite(&api.IngressGatewayConfigEntry{
		Kind: api.IngressGateway,
		Name: nameIG,

		Listeners: []api.IngressListener{
			{
				Port:     portWildcard,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name: "*",
					},
				},
				TLS: &api.GatewayTLSConfig{
					Enabled:       true,
					TLSMinVersion: "TLSv1_2",
					SDS: &api.GatewayTLSSDSConfig{
						ClusterName:  nameSDSCluster,
						CertResource: certResourceWildcard,
					},
				},
			},
			{
				Port:     portOther,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name:  libservice.StaticServerServiceName,
						Hosts: []string{hostnameWWW},
						TLS: &api.GatewayServiceTLSConfig{
							SDS: &api.GatewayTLSSDSConfig{
								ClusterName:  nameSDSCluster,
								CertResource: hostnameWWW,
							},
						},
					},
					{
						Name:  libservice.StaticServer2ServiceName,
						Hosts: []string{hostnameFoo},
						TLS: &api.GatewayServiceTLSConfig{
							SDS: &api.GatewayTLSSDSConfig{
								ClusterName:  nameSDSCluster,
								CertResource: hostnameFoo,
							},
						},
					},
				},
				TLS: &api.GatewayTLSConfig{
					Enabled:       true,
					TLSMinVersion: "TLSv1_2",
				},
			},
		},
	}))

	const staticClusterJSONKey = "envoy_extra_static_clusters_json"

	// register sds cluster as per https://developer.hashicorp.com/consul/docs/connect/gateways/ingress-gateway#configure-static-sds-cluster-s
	require.NoError(t, cluster.Servers()[0].GetClient().Agent().ServiceRegister(
		&api.AgentServiceRegistration{
			Kind: api.ServiceKindIngressGateway,
			Name: nameIG,
			Proxy: &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					// LOGICAL_DNS because we need to use a hostname
					// WARNING: this JSON is *very* sensitive and not well-checked.
					// bad values can lead to envoy not bootstrapping properly
					staticClusterJSONKey: fmt.Sprintf(`
{
  "name": "%s",
  "connect_timeout": "5s",
  "http2_protocol_options": {},
  "type": "LOGICAL_DNS",
  "load_assignment": {
    "cluster_name": "%s",
    "endpoints": [
      {
        "lb_endpoints": [
          {
            "endpoint": {
              "address": {
                "socket_address": {
                  "address": "%s",
                  "port_value": %d
                }
              }
            }
          }
        ]
      }
    ]
  }
}`, nameSDSCluster, nameSDSCluster, sdsServerContainerName, sdsServerPort),
				},
			},
		},
	))

	igw, err := libservice.NewGatewayServiceReg(context.Background(), libservice.GatewayConfig{
		Name: nameIG,
		Kind: "ingress",
	}, cluster.Servers()[0], false)
	require.NoError(t, err)

	// create s1
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(
		cluster.Clients()[0],
		&libservice.ServiceOpts{
			Name:     nameS1,
			ID:       nameS1,
			HTTPPort: 8080,
			GRPCPort: 8079,
		},
	)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, nameS1, nil)

	// create s2
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(
		cluster.Clients()[1],
		&libservice.ServiceOpts{
			Name:     nameS2,
			ID:       nameS2,
			HTTPPort: 8080,
			GRPCPort: 8079,
		},
	)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, nameS2, nil)

	tests := func(t *testing.T) {
		t.Run("ensure HTTP response with cert *.ingress.consul", func(t *testing.T) {
			port := portWildcard
			reqHost := fmt.Sprintf("%s.ingress.consul:%d", libservice.StaticServerServiceName, port)
			portMapped, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", port)),
			)

			httpClient := httpClientWithCA(t, reqHost, string(rootPEM))
			urlbase := fmt.Sprintf("https://%s", reqHost)
			resp := mappedHTTPGET(t, urlbase, portMapped.Int(), nil, nil, httpClient)
			defer resp.Body.Close()

			require.Equal(t, 1, len(resp.TLS.PeerCertificates))
			require.Equal(t, 1, len(resp.TLS.PeerCertificates[0].DNSNames))
			assert.Equal(t, "*.ingress.consul", resp.TLS.PeerCertificates[0].DNSNames[0])
		})

		t.Run("listener 2: ensure HTTP response with cert www.example.com", func(t *testing.T) {
			port := portOther
			reqHost := fmt.Sprintf("%s:%d", hostnameWWW, port)
			portMapped, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", port)),
			)

			httpClient := httpClientWithCA(t, reqHost, string(rootPEM))
			urlbase := fmt.Sprintf("https://%s", reqHost)
			resp := mappedHTTPGET(t, urlbase, portMapped.Int(), nil, nil, httpClient)
			defer resp.Body.Close()

			require.Equal(t, 1, len(resp.TLS.PeerCertificates))
			require.Equal(t, 1, len(resp.TLS.PeerCertificates[0].DNSNames))
			assert.Equal(t, hostnameWWW, resp.TLS.PeerCertificates[0].DNSNames[0])
		})

		t.Run("listener 2: ensure HTTP response with cert foo.example.com", func(t *testing.T) {
			port := portOther
			reqHost := fmt.Sprintf("%s:%d", hostnameFoo, port)
			portMapped, _ := cluster.Servers()[0].GetPod().MappedPort(
				context.Background(),
				nat.Port(fmt.Sprintf("%d/tcp", port)),
			)

			httpClient := httpClientWithCA(t, reqHost, string(rootPEM))
			urlbase := fmt.Sprintf("https://%s", reqHost)
			resp := mappedHTTPGET(t, urlbase, portMapped.Int(), nil, nil, httpClient)
			defer resp.Body.Close()

			require.Equal(t, 1, len(resp.TLS.PeerCertificates))
			require.Equal(t, 1, len(resp.TLS.PeerCertificates[0].DNSNames))
			assert.Equal(t, hostnameFoo, resp.TLS.PeerCertificates[0].DNSNames[0])
		})
	}

	t.Run("pre-upgrade", func(t *testing.T) {
		tests(t)
	})

	if t.Failed() {
		t.Fatal("failing fast: failed assertions pre-upgrade")
	}

	// Upgrade the cluster to utils.TargetVersion
	t.Logf("Upgrade to version %s", utils.TargetVersion)
	err = cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion)
	require.NoError(t, err)
	require.NoError(t, igw.Restart())

	t.Run("post-upgrade", func(t *testing.T) {
		tests(t)
	})
}

// createSDSServer builds and runs a test SDS server in the given cluster.
// It is built from files in .../test-sds-server, shared with the BATS tests.
// This includes some pre-generated certs for various scenarios.
//
// It returns the name of the container (which will also be the hostname), and
// the root CA's cert in PEM encoding
func createSDSServer(t *testing.T, cluster *libcluster.Cluster) (containerName string, rootPEM []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()

	sdsServerFilesPath, err := filepath.Abs("../../../connect/envoy/test-sds-server/")
	require.NoError(t, err)

	// TODO: we should probably just generate these certs on every boot
	certPath := filepath.Join(sdsServerFilesPath, "/certs")

	rootPEMf, err := os.Open(filepath.Join(certPath, "ca-root.crt"))
	require.NoError(t, err)

	rootPEM, err = io.ReadAll(rootPEMf)
	require.NoError(t, err)

	containerName = utils.RandName(fmt.Sprintf("%s-test-sds-server", cluster.Servers()[0].GetDatacenter()))

	_, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "consul-sds-server",
			Name:  containerName,
			Networks: []string{
				cluster.NetworkName,
			},
			ExposedPorts: []string{
				fmt.Sprintf("%d/tcp", sdsServerPort),
			},
			Mounts: []testcontainers.ContainerMount{
				{
					Source: testcontainers.DockerBindMountSource{
						HostPath: certPath,
					},
					Target:   "/certs",
					ReadOnly: true,
				},
			},
			WaitingFor: wait.ForLog("").WithStartupTimeout(60 * time.Second),
		},
	})
	require.NoError(t, err, "create SDS server container")
	return containerName, rootPEM
}
