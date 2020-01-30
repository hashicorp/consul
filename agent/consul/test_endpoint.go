package consul

import (
	"context"

	"github.com/hashicorp/consul/agent/agentpb"
)

// GRPCTest is a gRPC handler object for the agentpb.Test service. It's only
// used for testing gRPC plumbing details and never exposed in a running Consul
// server.
type GRPCTest struct {
	srv *Server
}

// Test is the gRPC Test.Test endpoint handler.
func (t *GRPCTest) Test(ctx context.Context, req *agentpb.TestRequest) (*agentpb.TestResponse, error) {
	if req.Datacenter != "" && req.Datacenter != t.srv.config.Datacenter {
		conn, err := t.srv.grpcClient.GRPCConn(req.Datacenter)
		if err != nil {
			return nil, err
		}

		t.srv.logger.Debug("GRPC test server conn state %s", conn.GetState())

		// Open a Test call to the remote DC.
		client := agentpb.NewTestClient(conn)
		return client.Test(ctx, req)
	}

	return &agentpb.TestResponse{ServerName: t.srv.config.NodeName}, nil
}
