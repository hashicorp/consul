package command

import (
	"flag"
	"github.com/armon/consul-api"
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

// HTTPAddrFlag returns a pointer to a string that will be populated
// when the given flagset is parsed with the HTTP address of the Consul.
func HTTPAddrFlag(f *flag.FlagSet) *string {
	return f.String("http-addr", "127.0.0.1:8500",
		"HTTP address of the Consul agent")
}

// HTTPClient returns a new Consul HTTP client with the given address.
func HTTPClient(addr string) (*consulapi.Client, error) {
	return HTTPClientDC(addr, "")
}

// HTTPClientDC returns a new Consul HTTP client with the given address and datacenter
func HTTPClientDC(addr, dc string) (*consulapi.Client, error) {
	conf := consulapi.DefaultConfig()
	conf.Address = addr
	conf.Datacenter = dc
	return consulapi.NewClient(conf)
}
