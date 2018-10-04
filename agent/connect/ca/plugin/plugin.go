package plugin

import (
	"context"
	"net/rpc"

	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

// ProviderPlugin implements plugin.Plugin for initializing a plugin
// server and client for both net/rpc and gRPC.
type ProviderPlugin struct {
	Impl ca.Provider
}

func (p ProviderPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &providerPluginRPCServer{impl: p.Impl}, nil
}

func (ProviderPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &providerPluginRPCClient{client: c}, nil
}

func (p ProviderPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	RegisterCAServer(s, &providerPluginGRPCServer{impl: p.Impl})
	return nil
}

func (ProviderPlugin) GRPCClient(doneCtx context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &providerPluginGRPCClient{
		client:     NewCAClient(c),
		clientConn: c,
		doneCtx:    doneCtx,
	}, nil
}

// Verification
var _ plugin.Plugin = ProviderPlugin{}
var _ plugin.GRPCPlugin = ProviderPlugin{}
