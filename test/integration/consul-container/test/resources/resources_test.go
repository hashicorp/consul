package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
)

func TestResources(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 2,
		BuildOpts:  &libcluster.BuildOptions{Datacenter: "dc1"},
	})

	// This test specifically targets a follower because:
	//
	//	1. It ensures leader-forwarding works (as only leaders can process writes).
	//	2. It exercises the full Raft replication and FSM flow (as writes will only
	//	   be readable once they are replicated to the follower).
	followers, err := cluster.Followers()
	require.NoError(t, err)
	client := pbresource.NewResourceServiceClient(followers[0].GetGRPCConn())

	// Write a Node and HealthStatus resource. We want to observe the node health
	// controller running (to exercise the full controller runtime machinery) and
	// later, observe the deletion of the Node cascading to the HealthStatus.
	nodeRsp, err := client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeNode,
				Tenancy: tenancyDefault,
				Name:    "node1",
			},
			Data: any(t, &pbcatalog.Node{
				Addresses: []*pbcatalog.NodeAddress{
					{Host: "127.0.0.1"},
				},
			}),
		},
	})
	require.NoError(t, err)

	statusRsp, err := client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeHealthStatus,
				Tenancy: tenancyDefault,
				Name:    "node1-http",
			},
			Owner: nodeRsp.Resource.Id,
			Data: any(t, &pbcatalog.HealthStatus{
				Type:   "http",
				Status: pbcatalog.Health_HEALTH_MAINTENANCE,
			}),
		},
	})
	require.NoError(t, err)

	// Check that the node health controller ran and updated the node's status.
	//
	// We don't actually care _what_ it wrote to the status in this test, as we're
	// just trying to exercise the controller runtime.
	retry.Run(t, func(r *retry.R) {
		readRsp, err := client.Read(ctx, &pbresource.ReadRequest{
			Id: nodeRsp.Resource.Id,
		})
		require.NoError(r, err)
		require.Contains(r, readRsp.Resource.Status, "consul.io/node-health")
	})

	// Delete the node and check it cascades to also delete the HealthStatus.
	_, err = client.Delete(ctx, &pbresource.DeleteRequest{Id: nodeRsp.Resource.Id})
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		_, err = client.Read(ctx, &pbresource.ReadRequest{Id: statusRsp.Resource.Id})
		require.Error(r, err)
		require.Equal(r, codes.NotFound.String(), status.Code(err).String())
	})
}

func any(t *testing.T, m protoreflect.ProtoMessage) *anypb.Any {
	t.Helper()

	any, err := anypb.New(m)
	require.NoError(t, err)

	return any
}

var (
	tenancyDefault = &pbresource.Tenancy{
		Partition: "default",
		PeerName:  "local",
		Namespace: "default",
	}

	typeNode = &pbresource.Type{
		Group:        "catalog",
		GroupVersion: "v1alpha1",
		Kind:         "Node",
	}

	typeHealthStatus = &pbresource.Type{
		Group:        "catalog",
		GroupVersion: "v1alpha1",
		Kind:         "HealthStatus",
	}
)
