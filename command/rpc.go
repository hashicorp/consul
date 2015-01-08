package command

import (
	"flag"
	"os"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/agent"
)

// RPCAddrEnvName defines an environment variable name which sets
// an RPC address if there is no -rpc-addr specified.
const RPCAddrEnvName = "CONSUL_RPC_ADDR"

// RPCAddrFlag returns a pointer to a string that will be populated
// when the given flagset is parsed with the RPC address of the Consul.
func RPCAddrFlag(f *flag.FlagSet) *string {
	defaultRPCAddr := os.Getenv(RPCAddrEnvName)
	if defaultRPCAddr == "" {
		defaultRPCAddr = "127.0.0.1:8400"
	}
	return f.String("rpc-addr", defaultRPCAddr,
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
	switch {
	case len(os.Getenv("CONSUL_HTTP_ADDR")) > 0:
		conf.Address = os.Getenv("CONSUL_HTTP_ADDR")
	default:
		conf.Address = addr
	}
	conf.Datacenter = dc
	return consulapi.NewClient(conf)
}
