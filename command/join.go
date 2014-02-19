package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

// JoinCommand is a Command implementation that tells a running Consul
// agent to join another.
type JoinCommand struct {
	Ui cli.Ui
}

func (c *JoinCommand) Help() string {
	helpText := `
Usage: consul join [options] address ...

  Tells a running Consul agent (with "consul agent") to join the cluster
  by specifying at least one existing member.

Options:

  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
  -wan                      Joins a server to another server in the WAN pool
`
	return strings.TrimSpace(helpText)
}

func (c *JoinCommand) Run(args []string) int {
	var wan bool

	cmdFlags := flag.NewFlagSet("join", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&wan, "wan", false, "wan")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	addrs := cmdFlags.Args()
	if len(addrs) == 0 {
		c.Ui.Error("At least one address to join must be specified.")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	defer client.Close()

	n, err := client.Join(addrs, wan)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error joining the cluster: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf(
		"Successfully joined cluster by contacting %d nodes.", n))
	return 0
}

func (c *JoinCommand) Synopsis() string {
	return "Tell Consul agent to join cluster"
}
