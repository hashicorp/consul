package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

// LeaveCommand is a Command implementation that instructs
// the Consul agent to gracefully leave the cluster
type LeaveCommand struct {
	Ui cli.Ui
}

func (c *LeaveCommand) Help() string {
	helpText := `
Usage: consul leave

  Causes the agent to gracefully leave the Consul cluster and shutdown.

Options:

  -rpc-addr=127.0.0.1:8400 RPC address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *LeaveCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("leave", flag.ContinueOnError)
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

	if err := client.Leave(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error leaving: %s", err))
		return 1
	}

	c.Ui.Output("Graceful leave complete")
	return 0
}

func (c *LeaveCommand) Synopsis() string {
	return "Gracefully leaves the Consul cluster and shuts down"
}
