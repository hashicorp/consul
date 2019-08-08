package flags

import (
	"flag"
	"os"

	"github.com/hashicorp/consul/api"
)

type ConnectFlags struct {
	proxyID    StringValue
	proxyToken StringValue
	sidecarFor StringValue
}

func (f *ConnectFlags) Init() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)

	fs.Var(&f.proxyID, "proxy-id",
		"The proxy's ID on the local agent. This can also be specified via the "+
			"CONNECT_PROXY_ID environment variable.")

	fs.Var(&f.proxyToken, "proxy-token",
		"ACL token to use in the request. This can also be specified via the "+
			"CONNECT_PROXY_TOKEN environment variable. If unspecified, the query will "+
			"default to the token of the Consul agent at the HTTP address.")

	fs.Var(&f.sidecarFor, "sidecar-for",
		"The ID of a service instance on the local agent that this proxy should "+
			"become a sidecar for. It requires that the proxy service is registered "+
			"with the agent as a connect-proxy with Proxy.DestinationServiceID set "+
			"to this value. If more than one such proxy is registered it will fail. "+
			"This can also be specified via the CONNECT_SIDECAR_FOR environment variable.")

	return fs
}

func (f *ConnectFlags) ProxyID() string {
	return f.proxyID.String()
}

func (f *ConnectFlags) SetProxyID(v string) error {
	return f.proxyID.Set(v)
}

func (f *ConnectFlags) SidecarFor() string {
	return f.sidecarFor.String()
}

func (f *ConnectFlags) APIClient() (*api.Client, error) {
	c := api.DefaultConfig()

	f.LoadFromEnv(c)

	return api.NewClient(c)
}

func (f *ConnectFlags) LoadFromEnv(c *api.Config) {
	// Load from env vars if they're set
	if f.proxyID.String() == "" {
		f.proxyID.Set(os.Getenv("CONNECT_PROXY_ID"))
	}
	// TODO(mike): is it even possible to check (c *cmd).http HTTPFlags from here?
	/*
		if f.proxyToken.String() == "" && c.http.Token() == "" && c.http.TokenFile() == "" {
			// Extra check needed since CONSUL_HTTP_TOKEN has not been consulted yet but
			// calling SetToken with empty will force that to override the
			if proxyToken := os.Getenv("CONNECT_PROXY_TOKEN"); proxyToken != "" {
				c.http.SetToken(proxyToken)
			}
		}
	*/
	if f.sidecarFor.String() == "" {
		f.sidecarFor.Set(os.Getenv("CONNECT_SIDECAR_FOR"))
	}

	// TODO(mike): Does merging onto config like this need to happen?
	// f.proxyToken.Merge(&c.Token)
}
