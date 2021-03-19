package redirecttraffic

import (
	"flag"
	"fmt"

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

	// flags
	proxyUID string

	provider iptables.Provider
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.proxyUID, "proxy-uid", "", "The user ID of the proxy to exclude from traffic redirection.")

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

	cfg := iptables.Config{ProxyUserID: c.proxyUID}
	if c.provider != nil {
		cfg.IptablesProvider = c.provider
	}

	err := iptables.Setup(cfg)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error setting up iptables rules: %s", err.Error()))
		return 1
	}

	c.UI.Info("Successfully applied iptables rules")
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Applies iptables rules for traffic redirection"
const help = `
Usage: consul connect iptables [options]

  Applies iptables rules for inbound and outbound traffic redirection.

  Requires iptables command line utility be installed separately.

  Example:

    $ consul connect iptables -proxy-uid 1234
`
