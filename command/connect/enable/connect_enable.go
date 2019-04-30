package enable

import (
	"flag"
	"fmt"
	"io"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	service      string
	protocol     string
	sidecarProxy bool

	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	c.flags.BoolVar(&c.sidecarProxy, "sidecar-proxy", false, "Whether the service should have a Sidecar Proxy by default")
	c.flags.StringVar(&c.service, "service", "", "The service to enable connect for")
	c.flags.StringVar(&c.protocol, "protocol", "", "The protocol spoken by the service")
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.service == "" {
		c.UI.Error("Must specify the -service parameter")
		return 1
	}

	entry := &api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     c.service,
		Protocol: c.protocol,
		Connect: api.ConnectConfiguration{
			SidecarProxy: c.sidecarProxy,
		},
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	written, _, err := client.ConfigEntries().Set(entry, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing config entry %q / %q: %v", entry.GetKind(), entry.GetName(), err))
		return 1
	}

	if !written {
		c.UI.Error(fmt.Sprintf("Config entry %q / %q not updated", entry.GetKind(), entry.GetName()))
		return 1
	}

	// TODO (mkeeler) should we output anything when successful
	return 0

}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Sets some simple Connect related configuration for a service"
const help = `
Usage: consul connect enable -service <service name> [options]

  Sets up some Connect related service defaults.

  Example:

    $ consul connect enable -service web -protocol http -sidecar-proxy true
`
