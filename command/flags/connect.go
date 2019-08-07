package flags

import (
	"flag"
	"os"

	"github.com/hashicorp/consul/api"
)

type ConnectFlags struct {
	proxyID    StringValue
	sidecarFor StringValue
}

func (f *ConnectFlags) Init() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)

	fs.Var(&f.proxyID, "proxy-id",
		"The proxy's ID on the local agent. This can also be specified via the "+
			"CONNECT_PROXY_ID environment variable.")

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
	if f.sidecarFor.String() == "" {
		f.sidecarFor.Set(os.Getenv("CONNECT_SIDECAR_FOR"))
	}
}
