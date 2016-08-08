package command

import (
	"flag"
	"os"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/agent"
)

const (
	// RPCAddrEnvName defines an environment variable name which sets
	// an RPC address if there is no -rpc-addr specified.
	RPCAddrEnvName = "CONSUL_RPC_ADDR"

	// HTTPAddrEnvName defines an environment variable name which sets
	// the HTTP address if there is no -http-addr specified.
	HTTPAddrEnvName = "CONSUL_HTTP_ADDR"

	// CONSUL_TOKEN defines an environment variable name wihich sets
	// the token if there is no -token specified.
	TokenEnvName = "CONSUL_TOKEN"
)

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
	defaultHTTPAddr := os.Getenv(HTTPAddrEnvName)
	if defaultHTTPAddr == "" {
		defaultHTTPAddr = "127.0.0.1:8500"
	}
	return f.String("http-addr", defaultHTTPAddr,
		"HTTP address of the Consul agent")
}

// HTTPClient returns a new Consul HTTP client with the given address.
func HTTPClient(addr string) (*consulapi.Client, error) {
	return HTTPClientConfig(func(c *consulapi.Config) {
		c.Address = addr
	})
}

func TokenFlag(f *flag.FlagSet) *string {
	defaultToken := os.Getenv(TokenEnvName)
	return f.String("token", defaultToken, "ACL token")
}

// HTTPClientConfig is used to return a new API client and modify its
// configuration by passing in a config modifier function.
func HTTPClientConfig(fn func(c *consulapi.Config)) (*consulapi.Client, error) {
	conf := consulapi.DefaultConfig()
	fn(conf)
	return consulapi.NewClient(conf)
}
