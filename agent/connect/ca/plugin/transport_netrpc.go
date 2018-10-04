package plugin

import (
	"net/rpc"

	"github.com/hashicorp/consul/agent/connect/ca"
)

// providerPluginRPCServer implements a net/rpc backed transport for
// an underlying implementation of a ca.Provider. The server side is the
// plugin binary itself.
type providerPluginRPCServer struct {
	impl ca.Provider
}

func (p *providerPluginRPCServer) Configure(args *ConfigureRPCRequest, _ *struct{}) error {
	return p.impl.Configure(args.ClusterId, args.IsRoot, args.RawConfig)
}

func (p *providerPluginRPCServer) GenerateRoot(struct{}, *struct{}) error {
	return p.impl.GenerateRoot()
}

// providerPluginRPCClient implements a net/rpc backed transport for
// an underlying implementation of a ca.Provider. The client side is the
// software calling into the plugin binary over rpc.
//
// This implements ca.Provider.
type providerPluginRPCClient struct {
	client *rpc.Client
}

func (p *providerPluginRPCClient) Configure(
	clusterId string,
	isRoot bool,
	rawConfig map[string]interface{}) error {
	return p.client.Call("Plugin.Configure", &ConfigureRPCRequest{
		ClusterId: clusterId,
		IsRoot:    isRoot,
		RawConfig: rawConfig,
	}, &struct{}{})
}

// Verification
// var _ ca.Provider = &providerPluginRPCClient{}

//-------------------------------------------------------------------
// Structs for net/rpc request and response

type ConfigureRPCRequest struct {
	ClusterId string
	IsRoot    bool
	RawConfig map[string]interface{}
}
