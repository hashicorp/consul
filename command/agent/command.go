package agent

import (
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

// Command is a Command implementation that runs a Serf agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	Ui         cli.Ui
	ShutdownCh <-chan struct{}
}

func (c *Command) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	conf := DefaultConfig()
	agent, err := Create(conf)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error starting agent: %s", err))
		return 1
	}
	defer agent.Shutdown()

	select {
	case <-c.ShutdownCh:
		return 0
	}
	return 0
}

func (c *Command) Synopsis() string {
	return "Runs a Consul agent"
}

func (c *Command) Help() string {
	helpText := `
Usage: consul agent [options]

  Starts the Consul agent and runs until an interrupt is received. The
  agent represents a single node in a cluster. An agent can also serve
  as a server by configuraiton.

Options:

  -rpc-addr=127.0.0.1:7373 Address to bind the RPC listener.
`
	return strings.TrimSpace(helpText)
}
