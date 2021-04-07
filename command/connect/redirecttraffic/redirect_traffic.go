package redirecttraffic

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

	c := &cmd{
		UI: ui,
	}
	c.init()
	return c
}

type cmd struct {
	UI     cli.Ui
	flags  *flag.FlagSet
	http   *flags.HTTPFlags
	help   string
	client *api.Client

	// Flags.
	proxyUID string
	proxyID  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.proxyUID, "proxy-uid", "", "The user ID of the proxy to exclude from traffic redirection.")
	c.flags.StringVar(&c.proxyID, "proxy-id", "", "The service ID of the proxy service registered with Consul.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.NamespaceFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.proxyUID == "" {
		c.UI.Error("-proxy-uid is required")
		return 1
	}

	cfg, err := c.generateConfigFromFlags()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to create configuration to apply traffic redirection rules: %s", err))
		return 1
	}

	err = iptables.Setup(cfg)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error setting up traffic redirection rules: %s", err.Error()))
		return 1
	}

	c.UI.Info("Successfully applied traffic redirection rules")
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

// todo: add docs
func (c *cmd) generateConfigFromFlags() (iptables.Config, error) {
	cfg := iptables.Config{ProxyUserID: c.proxyUID}

	// When proxyID is provided, we set up cfg with values
	// from proxy's service registration in Consul.
	if c.proxyID != "" {
		var err error
		if c.client == nil {
			c.client, err = c.http.APIClient()
			if err != nil {
				return iptables.Config{}, fmt.Errorf("error creating Consul API client: %s", err)
			}
		}

		svc, _, err := c.client.Agent().Service(c.proxyID, nil)
		if err != nil {
			return iptables.Config{}, fmt.Errorf("failed to fetch proxy service from Consul Agent: %s", err)
		}

		cfg.ProxyInboundPort = svc.Port

		// todo: change once it's configurable
		cfg.ProxyOutboundPort = xds.TProxyOutboundPort
	}

	return cfg, nil
}

const synopsis = "Applies iptables rules for traffic redirection"
const help = `
Usage: consul connect redirect-traffic [options]

  Applies iptables rules for inbound and outbound traffic redirection.

  Requires iptables command line utility be installed separately.

  Example:

    $ consul connect redirect-traffic -proxy-uid 1234
`
