package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

// ReloadCommand is a Command implementation that instructs
// the Consul agent to reload configurations
type ReloadCommand struct {
	Ui cli.Ui
}

func (c *ReloadCommand) Help() string {
	helpText := `
Usage: consul reload

  Causes the agent to reload configurations. This can be used instead
  of sending the SIGHUP signal to the agent.

Options:

  -rpc-addr=127.0.0.1:8400 RPC address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *ReloadCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("reload", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	defer client.Close()

	if err := client.Reload(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error reloading: %s", err))
		return 1
	}

	c.Ui.Output("Configuration reload triggered")
	return 0
}

func (c *ReloadCommand) Synopsis() string {
	return "Triggers the agent to reload configuration files"
}
