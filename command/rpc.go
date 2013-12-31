package command

import (
	"flag"
	"github.com/hashicorp/consul/command/agent"
)

// RPCAddrFlag returns a pointer to a string that will be populated
// when the given flagset is parsed with the RPC address of the Consul.
func RPCAddrFlag(f *flag.FlagSet) *string {
	return f.String("rpc-addr", "127.0.0.1:8400",
		"RPC address of the Consul agent")
}

// RPCClient returns a new Consul RPC client with the given address.
func RPCClient(addr string) (*agent.RPCClient, error) {
	return agent.NewRPCClient(addr)
}
