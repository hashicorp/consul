package leave

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

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	nonFlagArgs := c.flags.Args()
	if len(nonFlagArgs) > 0 {
		c.UI.Error(fmt.Sprintf("Error found unexpected args: %v", nonFlagArgs))
		c.UI.Output(c.Help())
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if err := client.Agent().Leave(); err != nil {
		c.UI.Error(fmt.Sprintf("Error leaving: %s", err))
		return 1
	}

	c.UI.Output("Graceful leave complete")
	return 0
}

func (c *cmd) Synopsis() string {
	return "Gracefully leaves the Consul cluster and shuts down"
}

func (c *cmd) Help() string {
	return c.usage
}

const usage = `Usage: consul leave [options]

  Causes the agent to gracefully leave the Consul cluster and shutdown.`
