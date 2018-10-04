package plugin

import (
	"net/rpc"

	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/go-plugin"
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

// Verification
var _ plugin.Plugin = ProviderPlugin{}
