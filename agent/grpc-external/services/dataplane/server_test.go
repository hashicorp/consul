package dataplane

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

func testClient(t *testing.T, server *Server) pbdataplane.DataplaneServiceClient {
	t.Helper()

	addr := testutils.RunTestServer(t, server)

	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbdataplane.NewDataplaneServiceClient(conn)
}
