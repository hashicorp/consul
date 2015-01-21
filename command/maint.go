package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

// MaintCommand is a Command implementation that enables or disables
// node or service maintenance mode.
type MaintCommand struct {
	Ui cli.Ui
}

func (c *MaintCommand) Help() string {
	helpText := `
Usage: consul maint [options]

  Places a node or service into maintenance mode. During maintenance mode,
  the node or service will be excluded from all queries through the DNS
  or API interfaces, effectively taking it out of the pool of available
  nodes. This is done by registering an additional critical health check.

  When enabling maintenance mode for a node or service, you may optionally
  specify a reason string. This string will appear in the "Notes" field
  of the critical health check which is registered against the node or
  service. If no reason is provided, a default value will be used.

  Maintenance mode is persistent, and will be restored in the event of an
  agent restart. It is therefore required to disable maintenance mode on
  a given node or service before it will be placed back into the pool.

  By default, we operate on the node as a whole. By specifying the
  "-service" argument, this behavior can be changed to enable or disable
  only a specific service.

Options:

  -enable                    Enable maintenance mode.
  -disable                   Disable maintenance mode.
  -reason=<string>           Text string describing the maintenance reason
  -service=<serviceID>       A specific service ID to enable/disable
  -token=""                  ACL token to use. Defaults to that of agent.
  -http-addr=127.0.0.1:8500  HTTP address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *MaintCommand) Run(args []string) int {
	var enable bool
	var disable bool
	var reason string
	var serviceID string
	var token string

	cmdFlags := flag.NewFlagSet("maint", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.BoolVar(&enable, "enable", false, "enable maintenance mode")
	cmdFlags.BoolVar(&disable, "disable", false, "disable maintenance mode")
	cmdFlags.StringVar(&reason, "reason", "", "maintenance reason")
	cmdFlags.StringVar(&serviceID, "service", "", "service maintenance")
	cmdFlags.StringVar(&token, "token", "", "")
	httpAddr := HTTPAddrFlag(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Ensure we don't have conflicting args
	if !enable && !disable {
		c.Ui.Error("One of -enable or -disable must be specified")
		return 1
	}
	if enable && disable {
		c.Ui.Error("Only one of -enable or -disable may be provided")
		return 1
	}
	if disable && reason != "" {
		c.Ui.Error("Reason may only be provided with -enable")
		return 1
	}

	// Create and test the HTTP client
	conf := api.DefaultConfig()
	conf.Address = *httpAddr
	conf.Token = token
	client, err := api.NewClient(conf)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	agent := client.Agent()
	if _, err := agent.NodeName(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	if enable {
		// Enable node maintenance
		if serviceID == "" {
			if err := agent.EnableNodeMaintenance(); err != nil {
				c.Ui.Error(fmt.Sprintf("Error enabling node maintenance: %s", err))
				return 1
			}
			c.Ui.Output("Node maintenance is now enabled")
			return 0
		}

		// Enable service maintenance
		if err := agent.EnableServiceMaintenance(serviceID); err != nil {
			c.Ui.Error(fmt.Sprintf("Error enabling service maintenance: %s", err))
			return 1
		}
		c.Ui.Output(fmt.Sprintf("Service maintenance is now enabled for %q", serviceID))
	}

	return 0
}

func (c *MaintCommand) Synopsis() string {
	return "Controls node or service maintenance mode"
}
