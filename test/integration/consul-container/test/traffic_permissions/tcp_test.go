// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tproxy

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"

	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/require"
)

const (
	echoPort                = 9999
	tcpPort                 = 8888
	staticServerVIP         = "240.0.0.1"
	staticServerReturnValue = "static-server"
	staticServerIdentity    = "static-server-identity"
)

var requestRetryTimer = &retry.Timer{Timeout: 20 * time.Second, Wait: 500 * time.Millisecond}

type trafficPermissionsCase struct {
	tp1                *pbauth.TrafficPermissions
	tp2                *pbauth.TrafficPermissions
	client1TCPSuccess  bool
	client1EchoSuccess bool
	client2TCPSuccess  bool
	client2EchoSuccess bool
}

func runTrafficPermissionsTests(t *testing.T, aclsEnabled bool, cases map[string]trafficPermissionsCase) {
	t.Parallel()

	cluster, resourceClient := createCluster(t, aclsEnabled)

	serverService, serverDataplane := createServerResources(t, resourceClient, cluster, cluster.Agents[1])
	client1Dataplane := createClientResources(t, resourceClient, cluster, serverService, cluster.Agents[2], 1)
	client2Dataplane := createClientResources(t, resourceClient, cluster, serverService, cluster.Agents[3], 2)

	assertDataplaneContainerState(t, client1Dataplane, "running")
	assertDataplaneContainerState(t, client2Dataplane, "running")
	assertDataplaneContainerState(t, serverDataplane, "running")

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			storeStaticServerTrafficPermissions(t, resourceClient, tc.tp1, 1)
			storeStaticServerTrafficPermissions(t, resourceClient, tc.tp2, 2)

			// We must establish a new TCP connection each time because TCP traffic permissions are
			// enforced at the connection level.
			retry.RunWith(requestRetryTimer, t, func(r *retry.R) {
				assertPassing(r, httpRequestToVirtualAddress, client1Dataplane, tc.client1TCPSuccess)
				assertPassing(r, echoToVirtualAddress, client1Dataplane, tc.client1EchoSuccess)
				assertPassing(r, httpRequestToVirtualAddress, client2Dataplane, tc.client2TCPSuccess)
				assertPassing(r, echoToVirtualAddress, client2Dataplane, tc.client2EchoSuccess)
			})
		})
	}
}

func TestTrafficPermission_TCP_DefaultDeny(t *testing.T) {
	cases := map[string]trafficPermissionsCase{
		"default deny": {
			tp1:                nil,
			client1TCPSuccess:  false,
			client1EchoSuccess: false,
			client2TCPSuccess:  false,
			client2EchoSuccess: false,
		},
		"allow everything": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								// IdentityName: "static-client-1-identity",
								Namespace: "default",
								Partition: "default",
								Peer:      "local",
							},
						},
					},
				},
			},
			client1TCPSuccess:  true,
			client1EchoSuccess: true,
			client2TCPSuccess:  true,
			client2EchoSuccess: true,
		},
		"allow tcp": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								// IdentityName: "static-client-1-identity",
								Namespace: "default",
								Partition: "default",
								Peer:      "local",
							},
						},
						DestinationRules: []*pbauth.DestinationRule{
							{
								PortNames: []string{"tcp"},
							},
						},
					},
				},
			},
			client1TCPSuccess:  true,
			client1EchoSuccess: false,
			client2TCPSuccess:  true,
			client2EchoSuccess: false,
		},
		"client 1 only": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								IdentityName: "static-client-1-identity",
								Namespace:    "default",
								Partition:    "default",
								Peer:         "local",
							},
						},
					},
				},
			},
			client1TCPSuccess:  true,
			client1EchoSuccess: true,
			client2TCPSuccess:  false,
			client2EchoSuccess: false,
		},
		"allow all exclude client 1": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Namespace: "default",
								Partition: "default",
								Peer:      "local",
								Exclude: []*pbauth.ExcludeSource{
									{
										IdentityName: "static-client-1-identity",
										Namespace:    "default",
										Partition:    "default",
										Peer:         "local",
									},
								},
							},
						},
					},
				},
			},
			client1TCPSuccess:  false,
			client1EchoSuccess: false,
			client2TCPSuccess:  true,
			client2EchoSuccess: true,
		},
		"deny takes precedence over allow": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_DENY,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								IdentityName: "static-client-1-identity",
								Namespace:    "default",
								Partition:    "default",
								Peer:         "local",
							},
						},
					},
				},
			},
			tp2: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								IdentityName: "static-client-1-identity",
								Namespace:    "default",
								Partition:    "default",
								Peer:         "local",
							},
						},
					},
				},
			},
			client1TCPSuccess:  false,
			client1EchoSuccess: false,
			client2TCPSuccess:  false,
			client2EchoSuccess: false,
		},
		"deny all exclude service + allow on that service": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_DENY,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Namespace: "default",
								Partition: "default",
								Peer:      "local",
								Exclude: []*pbauth.ExcludeSource{
									{
										IdentityName: "static-client-1-identity",
										Namespace:    "default",
										Partition:    "default",
										Peer:         "local",
									},
								},
							},
						},
					},
				},
			},
			tp2: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								IdentityName: "static-client-1-identity",
								Namespace:    "default",
								Partition:    "default",
								Peer:         "local",
							},
						},
					},
				},
			},
			client1TCPSuccess:  true,
			client1EchoSuccess: true,
			client2TCPSuccess:  false,
			client2EchoSuccess: false,
		},
	}

	runTrafficPermissionsTests(t, true, cases)
}

func TestTrafficPermission_TCP_DefaultAllow(t *testing.T) {
	cases := map[string]trafficPermissionsCase{
		"default allow": {
			tp1:                nil,
			client1TCPSuccess:  true,
			client1EchoSuccess: true,
			client2TCPSuccess:  true,
			client2EchoSuccess: true,
		},
		"allow everything": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Namespace: "default",
								Partition: "default",
								Peer:      "local",
							},
						},
					},
				},
			},
			client1TCPSuccess:  true,
			client1EchoSuccess: true,
			client2TCPSuccess:  true,
			client2EchoSuccess: true,
		},
		// TODO I don't like this behavior.
		"allow one protocol doesnt impact the other protocol": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Namespace: "default",
								Partition: "default",
								Peer:      "local",
							},
						},
						DestinationRules: []*pbauth.DestinationRule{
							{
								PortNames: []string{"tcp"},
							},
						},
					},
				},
			},
			client1TCPSuccess:  true,
			client1EchoSuccess: true,
			client2TCPSuccess:  true,
			client2EchoSuccess: true,
		},
		"allow something unrelated": {
			tp1: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: staticServerIdentity,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								IdentityName: "something-else",
								Namespace:    "default",
								Partition:    "default",
								Peer:         "local",
							},
						},
					},
				},
			},
			client1TCPSuccess:  false,
			client1EchoSuccess: false,
			client2TCPSuccess:  false,
			client2EchoSuccess: false,
		},
	}

	runTrafficPermissionsTests(t, false, cases)
}

func createServiceAndDataplane(t *testing.T, node libcluster.Agent, cluster *libcluster.Cluster, proxyID, serviceName string, httpPort, grpcPort int, serviceBindPorts []int) (*libcluster.ConsulDataplaneContainer, error) {
	leader, err := cluster.Leader()
	require.NoError(t, err)
	leaderIP := leader.GetIP()

	token := cluster.TokenBootstrap

	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	// Create a service and proxy instance
	svc, err := libservice.NewExampleService(context.Background(), serviceName, httpPort, grpcPort, node)
	if err != nil {
		return nil, err
	}
	deferClean.Add(func() {
		_ = svc.Terminate()
	})

	// Create Consul Dataplane
	dp, err := libcluster.NewConsulDataplane(context.Background(), proxyID, leaderIP, 8502, serviceBindPorts, node, true, token)
	require.NoError(t, err)
	deferClean.Add(func() {
		_ = dp.Terminate()
	})

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return dp, nil
}

func storeStaticServerTrafficPermissions(t *testing.T, resourceClient *rtest.Client, tp *pbauth.TrafficPermissions, i int) {
	id := &pbresource.ID{
		Name: fmt.Sprintf("static-server-tp-%d", i),
		Type: pbauth.TrafficPermissionsType,
	}
	if tp == nil {
		resourceClient.Delete(resourceClient.Context(t), &pbresource.DeleteRequest{
			Id: id,
		})
	} else {
		rtest.ResourceID(id).
			WithData(t, tp).
			Write(t, resourceClient)
	}
}

func createServerResources(t *testing.T, resourceClient *rtest.Client, cluster *libcluster.Cluster, node libcluster.Agent) (*pbresource.Resource, *libcluster.ConsulDataplaneContainer) {
	serverService := rtest.ResourceID(&pbresource.ID{
		Name: "static-server-service",
		Type: pbcatalog.ServiceType,
	}).
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"static-server"}},
			Ports: []*pbcatalog.ServicePort{
				{
					TargetPort:  "tcp",
					Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
					VirtualPort: 8888,
				},
				{
					TargetPort:  "echo",
					Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
					VirtualPort: 9999,
				},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
			VirtualIps: []string{"240.0.0.1"},
		}).Write(t, resourceClient)

	workloadPortMap := map[string]*pbcatalog.WorkloadPort{
		"tcp": {
			Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
		},
		"echo": {
			Port: 8078, Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
		},
		"mesh": {
			Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
		},
	}

	rtest.ResourceID(&pbresource.ID{
		Name: "static-server-workload",
		Type: pbcatalog.WorkloadType,
	}).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: node.GetIP()},
			},
			Ports:    workloadPortMap,
			Identity: staticServerIdentity,
		}).
		Write(t, resourceClient)

	rtest.ResourceID(&pbresource.ID{
		Name: staticServerIdentity,
		Type: pbauth.WorkloadIdentityType,
	}).
		Write(t, resourceClient)

	serverDataplane, err := createServiceAndDataplane(t, node, cluster, "static-server-workload", "static-server", 8080, 8079, []int{})
	require.NoError(t, err)

	return serverService, serverDataplane
}

func createClientResources(t *testing.T, resourceClient *rtest.Client, cluster *libcluster.Cluster, staticServerRef *pbresource.Resource, node libcluster.Agent, idx int) *libcluster.ConsulDataplaneContainer {
	prefix := fmt.Sprintf("static-client-%d", idx)
	rtest.ResourceID(&pbresource.ID{
		Name: prefix + "-service",
		Type: pbcatalog.ServiceType,
	}).
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{prefix}},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}).Write(t, resourceClient)

	workloadPortMap := map[string]*pbcatalog.WorkloadPort{
		"tcp": {
			Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
		},
		"mesh": {
			Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
		},
	}

	rtest.ResourceID(&pbresource.ID{
		Name: prefix + "-workload",
		Type: pbcatalog.WorkloadType,
	}).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: node.GetIP()},
			},
			Ports:    workloadPortMap,
			Identity: prefix + "-identity",
		}).
		Write(t, resourceClient)

	rtest.ResourceID(&pbresource.ID{
		Name: prefix + "-identity",
		Type: pbauth.WorkloadIdentityType,
	}).
		Write(t, resourceClient)

	rtest.ResourceID(&pbresource.ID{
		Name: prefix + "-proxy-configuration",
		Type: pbmesh.ProxyConfigurationType,
	}).
		WithData(t, &pbmesh.ProxyConfiguration{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"static-client"},
			},
			DynamicConfig: &pbmesh.DynamicConfig{
				Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			},
		}).
		Write(t, resourceClient)

	dp, err := createServiceAndDataplane(t, node, cluster, fmt.Sprintf("static-client-%d-workload", idx), "static-client", 8080, 8079, []int{})
	require.NoError(t, err)

	return dp
}

func createCluster(t *testing.T, aclsEnabled bool) (*libcluster.Cluster, *rtest.Client) {
	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 1,
		NumClients: 3,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
			ACLEnabled:             aclsEnabled,
		},
		Cmd: `-hcl=experiments=["resource-apis"] log_level="TRACE"`,
	})

	leader, err := cluster.Leader()
	require.NoError(t, err)
	client := pbresource.NewResourceServiceClient(leader.GetGRPCConn())
	resourceClient := rtest.NewClientWithACLToken(client, cluster.TokenBootstrap)

	return cluster, resourceClient
}

// assertDataplaneContainerState validates service container status
func assertDataplaneContainerState(t *testing.T, dataplane *libcluster.ConsulDataplaneContainer, state string) {
	containerStatus, err := dataplane.GetStatus()
	require.NoError(t, err)
	require.Equal(t, containerStatus, state, fmt.Sprintf("Expected: %s. Got %s", state, containerStatus))
}

func httpRequestToVirtualAddress(dp *libcluster.ConsulDataplaneContainer) (string, error) {
	addr := fmt.Sprintf("%s:%d", staticServerVIP, tcpPort)

	// Test that we can make a request to the virtual ip to reach the upstream.
	//
	// NOTE(pglass): This uses a workaround for DNS because I had trouble modifying
	// /etc/resolv.conf. There is a --dns option to docker run, but it
	// didn't seem to be exposed via testcontainers. I'm not sure if it would
	// do what I want. In any case, Docker sets up /etc/resolv.conf for certain
	// functionality so it seems better to leave DNS alone.
	//
	// But, that means DNS queries aren't redirected to Consul out of the box.
	// As a workaround, we `dig @localhost:53` which is iptables-redirected to
	// localhost:8600 where the Consul client responds with the virtual ip.
	//
	// In tproxy tests, Envoy is not configured with a unique listener for each
	// upstream. This means the usual approach for non-tproxy tests doesn't
	// work - where we send the request to a host address mapped in to Envoy's
	// upstream listener. Instead, we exec into the container and run curl.
	//
	// We must make this request with a non-envoy user. The envoy and consul
	// users are excluded from traffic redirection rules, so instead we
	// make the request as root.
	out, err := dp.Exec(
		context.Background(),
		[]string{"sudo", "sh", "-c", fmt.Sprintf(`
set -e
			curl -s "%s/debug?env=dump"
			`, addr),
		},
	)

	if err != nil {
		return out, fmt.Errorf("curl request to upstream virtual address %q\nerr = %v\nout = %s\nservice=%s", addr, err, out, dp.GetServiceName())
	}

	expected := fmt.Sprintf("FORTIO_NAME=%s", staticServerReturnValue)
	if !strings.Contains(out, expected) {
		return out, fmt.Errorf("expected %q to contain %q", out, expected)
	}

	return out, nil
}

func echoToVirtualAddress(dp *libcluster.ConsulDataplaneContainer) (string, error) {
	// Test that we can make a request to the virtual ip to reach the upstream.
	//
	// NOTE(pglass): This uses a workaround for DNS because I had trouble modifying
	// /etc/resolv.conf. There is a --dns option to docker run, but it
	// didn't seem to be exposed via testcontainers. I'm not sure if it would
	// do what I want. In any case, Docker sets up /etc/resolv.conf for certain
	// functionality so it seems better to leave DNS alone.
	//
	// But, that means DNS queries aren't redirected to Consul out of the box.
	// As a workaround, we `dig @localhost:53` which is iptables-redirected to
	// localhost:8600 where the Consul client responds with the virtual ip.
	//
	// In tproxy tests, Envoy is not configured with a unique listener for each
	// upstream. This means the usual approach for non-tproxy tests doesn't
	// work - where we send the request to a host address mapped in to Envoy's
	// upstream listener. Instead, we exec into the container and run curl.
	//
	// We must make this request with a non-envoy user. The envoy and consul
	// users are excluded from traffic redirection rules, so instead we
	// make the request as root.
	out, err := dp.Exec(
		context.Background(),
		[]string{"sudo", "sh", "-c", fmt.Sprintf(`
			set -e
			echo foo | nc %s %d
			`, staticServerVIP, echoPort),
		},
	)

	if err != nil {
		return out, fmt.Errorf("nc request to upstream virtual address %s:%d\nerr = %v\nout = %s\nservice=%s", staticServerVIP, echoPort, err, out, dp.GetServiceName())
	}

	if !strings.Contains(out, "foo") {
		return out, fmt.Errorf("expected %q to contain 'foo'", out)
	}

	return out, err
}

func assertPassing(t require.TestingT, fn func(*libcluster.ConsulDataplaneContainer) (string, error), dp *libcluster.ConsulDataplaneContainer, success bool) {
	_, err := fn(dp)
	if success {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
	}
}
