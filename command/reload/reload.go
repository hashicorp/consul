package reload

import (
	"flag"
	"fmt"

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
	usage string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.usage = flags.Usage(usage, c.flags, c.http.ClientFlags(), c.http.ServerFlags())
}

func (c *cmd) Synopsis() string {
	return "Triggers the agent to reload configuration files"
}

func (c *cmd) Help() string {
	return c.usage
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if err := client.Agent().Reload(); err != nil {
		c.UI.Error(fmt.Sprintf("Error reloading: %s", err))
		return 1
	}

	c.UI.Output("Configuration reload triggered")
	return 0
}

const usage = `Usage: consul reload

  Causes the agent to reload configurations. This can be used instead
  of sending the SIGHUP signal to the agent.`
