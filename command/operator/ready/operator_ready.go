package ready

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
	help  string
	flags *flag.FlagSet
	http  *flags.HTTPFlags

	// flags
	logLevel string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.logLevel, "logLevel", "",
		"The log level needed?")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	leader, err := client.Status().Leader()

	if err != nil {
		c.UI.Error(fmt.Sprintf("Error getting leader: %v", err))
		return 1
	}
	c.UI.Output(fmt.Sprintf("%v", leader))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Provides an indicator if ready after joining"
const help = `
Usage: consul operator ready [options]

The Ready operator command is used to verify the agent is ready
after it's joined a cluster
`
