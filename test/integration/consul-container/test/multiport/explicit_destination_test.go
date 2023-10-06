// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package multiport

import (
	"context"
	"fmt"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/stretchr/testify/require"
	"testing"

	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestMultiportService_Explicit makes sure two services in the same datacenter have connectivity
// with transparent proxy enabled.
//
// Steps:
//   - Create a single server cluster.
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Make sure a request from static-client to the virtual address (<svc>.virtual.consul) returns a
//     response from the upstream.
func TestMultiportService_Explicit(t *testing.T) {
	t.Parallel()

	cluster := createCluster(t)
	followers, err := cluster.Followers()
	require.NoError(t, err)
	client := pbresource.NewResourceServiceClient(followers[0].GetGRPCConn())
	resourceClient := rtest.NewClient(client)

	serverIP := cluster.Agents[1].GetIP()
	clientIP := cluster.Agents[2].GetIP()

	serverService := createServerServicesAndWorkloads(t, resourceClient, serverIP)
	createClientResources(t, resourceClient, serverService, clientIP)

	_, clientDataplane := createServices(t, cluster)

	_, port := clientDataplane.GetAddr()

	assertDataplaneContainerState(t, clientDataplane, "running")
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")
}

// createServices creates the static-client and static-server services with
// transparent proxy enabled. It returns a Service for the static-client.
func createServices(t *testing.T, cluster *libcluster.Cluster) (*libcluster.ConsulDataplaneContainer, *libcluster.ConsulDataplaneContainer) {
	n1 := cluster.Agents[1]

	// Create a service and dataplane
	serverDataplane, err := createServiceAndDataplane(t, n1, "static-server-workload", "static-server", 8080, 8079, []int{})
	require.NoError(t, err)

	n2 := cluster.Agents[2]
	// Create a service and dataplane
	clientDataplane, err := createServiceAndDataplane(t, n2, "static-client-workload", "static-client", 8080, 8079, []int{libcluster.ServiceUpstreamLocalBindPort})
	require.NoError(t, err)

	return serverDataplane, clientDataplane
}

func createServiceAndDataplane(t *testing.T, node libcluster.Agent, proxyID, serviceName string, httpPort, grpcPort int, serviceBindPorts []int) (*libcluster.ConsulDataplaneContainer, error) {
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
	dp, err := libcluster.NewConsulDataplane(context.Background(), proxyID, "0.0.0.0", 8502, serviceBindPorts, node)
	require.NoError(t, err)
	deferClean.Add(func() {
		_ = dp.Terminate()
	})

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return dp, nil
}

func createServerServicesAndWorkloads(t *testing.T, resourceClient *rtest.Client, ipAddress string) *pbresource.Resource {
	serverService := rtest.ResourceID(&pbresource.ID{
		Name: "static-server-service",
		Type: pbcatalog.ServiceType,
	}).WithData(t, &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"static-server"}},
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
		Name: "static-server-identity",
		Type: pbauth.WorkloadIdentityType,
	}).Write(t, resourceClient)

	rtest.ResourceID(&pbresource.ID{
		Name: "static-server-workload",
		Type: pbcatalog.WorkloadType,
	}).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: ipAddress},
			},
			Ports:    workloadPortMap,
			Identity: "static-server-identity",
		}).
		Write(t, resourceClient)
	return serverService
}

func createClientResources(t *testing.T, resourceClient *rtest.Client, staticServerResource *pbresource.Resource, ipAddress string) {
	rtest.ResourceID(&pbresource.ID{
		Name: "static-client-service",
		Type: pbcatalog.ServiceType,
	}).WithData(t, &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"static-client"}},
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
		Name: "static-client-workload",
		Type: pbcatalog.WorkloadType,
	}).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: ipAddress},
			},
			Ports:    workloadPortMap,
			Identity: "static-client-identity",
		}).
		Write(t, resourceClient)

	destId := staticServerResource.GetId()
	destRef := &pbresource.Reference{
		Type:    destId.Type,
		Tenancy: destId.Tenancy,
		Name:    destId.Name,
		Section: "",
	}
	rtest.ResourceID(&pbresource.ID{
		Name: "static-client-upstreams",
		Type: pbmesh.DestinationsType,
	}).
		WithData(t, &pbmesh.Destinations{
			Destinations: []*pbmesh.Destination{
				{
					DestinationRef:  destRef,
					DestinationPort: "tcp",
					ListenAddr: &pbmesh.Destination_IpPort{
						IpPort: &pbmesh.IPPortAddress{
							Ip:   "0.0.0.0",
							Port: libcluster.ServiceUpstreamLocalBindPort,
						},
					},
				},
			},
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"static-client"},
			},
		}).
		Write(t, resourceClient)
}

func createCluster(t *testing.T) *libcluster.Cluster {
	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 3,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
		},
		Cmd: `-hcl=experiments=["resource-apis"] log_level="TRACE"`,
	})

	return cluster
}

// assertDataplaneContainerState validates service container status
func assertDataplaneContainerState(t *testing.T, dataplane *libcluster.ConsulDataplaneContainer, state string) {
	containerStatus, err := dataplane.GetStatus()
	require.NoError(t, err)
	require.Equal(t, containerStatus, state, fmt.Sprintf("Expected: %s. Got %s", state, containerStatus))
}
