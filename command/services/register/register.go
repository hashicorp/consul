package register

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/services"
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

	// flags
	flagMeta map[string]string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.Var((*flags.FlagMapValue)(&c.flagMeta), "meta",
		"Metadata to set on the intention, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple meta fields.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Check for arg validation
	args = c.flags.Args()
	if len(args) == 0 {
		c.UI.Error("Service registration requires at least one argument.")
		return 1
	}

	svcs, err := services.ServicesFromFiles(args)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error: %s", err))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Create all the services
	for _, svc := range svcs {
		if err := client.Agent().ServiceRegister(svc); err != nil {
			c.UI.Error(fmt.Sprintf("Error registering service %q: %s",
				svc.Name, err))
			return 1
		}
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Register services with the local agent"
const help = `
Usage: consul services register [options] [FILE...]

  Register one or more services using the local agent API. Services can
  be registered from standard Consul configuration files (HCL or JSON) or
  using flags. The service is registered and the command returns. The caller
  must remember to call "consul services deregister" or a similar API to
  deregister the service when complete.

      $ consul services register web.json

  Additional flags and more advanced use cases are detailed below.
`
