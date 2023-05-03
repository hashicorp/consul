package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
)

// TestResources exercises the full Resource Service and Controller stack.
func TestResources(t *testing.T) {
	t.Parallel()

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 2,
		Cmd:        `-hcl=enable_dev_resources = true`,
		BuildOpts:  &libcluster.BuildOptions{Datacenter: "dc1"},
	})

	// Write an artist resource to a follower to exercise leader forwarding.
	followers, err := cluster.Followers()
	require.NoError(t, err)

	conn := followers[0].GetGRPCConn()
	client := pbresource.NewResourceServiceClient(conn)
	ctx := testutil.TestContext(t)

	artist, err := anypb.New(&pbdemov2.Artist{
		Name:  "Five Iron Frenzy",
		Genre: pbdemov2.Genre_GENRE_SKA,
	})
	require.NoError(t, err)

	writeRsp, err := client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type: &pbresource.Type{
					Group:        "demo",
					GroupVersion: "v2",
					Kind:         "artist",
				},
				Tenancy: &pbresource.Tenancy{
					Partition: "default",
					PeerName:  "local",
					Namespace: "default",
				},
				Name: "five-iron-frenzy",
			},
			Data: artist,
		},
	})
	require.NoError(t, err)

	// Wait for controller to run and update the artist's status. Also checks
	// leader to follower replication is working.
	retry.Run(t, func(r *retry.R) {
		readRsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: writeRsp.Resource.Id})
		require.NoError(r, err)
		require.NotNil(r, readRsp.Resource.Status)
		require.Contains(r, readRsp.Resource.Status, "consul.io/artist-controller")
	})

	// Controller will create albums for the artist.
	listRsp, err := client.List(ctx, &pbresource.ListRequest{
		Type: &pbresource.Type{
			Group:        "demo",
			GroupVersion: "v2",
			Kind:         "album",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			PeerName:  "local",
			Namespace: "default",
		},
	})
	require.NoError(t, err)
	require.Len(t, listRsp.Resources, 3)
}
