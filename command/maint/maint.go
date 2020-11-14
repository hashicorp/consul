package maint

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

// cmd is a Command implementation that enables or disables
// node or service maintenance mode.
type cmd struct {
	UI    cli.Ui
	help  string
	flags *flag.FlagSet
	http  *flags.HTTPFlags

	// flags
	enable    bool
	disable   bool
	reason    string
	serviceID string
}

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.enable, "enable", false,
		"Enable maintenance mode.")
	c.flags.BoolVar(&c.disable, "disable", false,
		"Disable maintenance mode.")
	c.flags.StringVar(&c.reason, "reason", "",
		"Text describing the maintenance reason.")
	c.flags.StringVar(&c.serviceID, "service", "",
		"Control maintenance mode for a specific service ID.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Ensure we don't have conflicting args
	if c.enable && c.disable {
		c.UI.Error("Only one of -enable or -disable may be provided")
		return 1
	}
	if !c.enable && c.reason != "" {
		c.UI.Error("Reason may only be provided with -enable")
		return 1
	}
	if !c.enable && !c.disable && c.serviceID != "" {
		c.UI.Error("Service requires either -enable or -disable")
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	a := client.Agent()

	if !c.enable && !c.disable {
		nodeName, err := a.NodeName()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
			return 1
		}

		// List mode - list nodes/services in maintenance mode
		checks, err := a.Checks()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error getting checks: %s", err))
			return 1
		}

		for _, check := range checks {
			if check.CheckID == "_node_maintenance" {
				c.UI.Output("Node:")
				c.UI.Output("  Name:   " + nodeName)
				c.UI.Output("  Reason: " + check.Notes)
				c.UI.Output("")
			} else if strings.HasPrefix(check.CheckID, "_service_maintenance:") {
				c.UI.Output("Service:")
				c.UI.Output("  ID:     " + check.ServiceID)
				c.UI.Output("  Reason: " + check.Notes)
				c.UI.Output("")
			}
		}

		return 0
	}

	if c.enable {
		// Enable node maintenance
		if c.serviceID == "" {
			if err := a.EnableNodeMaintenance(c.reason); err != nil {
				c.UI.Error(fmt.Sprintf("Error enabling node maintenance: %s", err))
				return 1
			}
			c.UI.Output("Node maintenance is now enabled")
			return 0
		}

		// Enable service maintenance
		if err := a.EnableServiceMaintenance(c.serviceID, c.reason); err != nil {
			c.UI.Error(fmt.Sprintf("Error enabling service maintenance: %s", err))
			return 1
		}
		c.UI.Output(fmt.Sprintf("Service maintenance is now enabled for %q", c.serviceID))
		return 0
	}

	if c.disable {
		// Disable node maintenance
		if c.serviceID == "" {
			if err := a.DisableNodeMaintenance(); err != nil {
				c.UI.Error(fmt.Sprintf("Error disabling node maintenance: %s", err))
				return 1
			}
			c.UI.Output("Node maintenance is now disabled")
			return 0
		}

		// Disable service maintenance
		if err := a.DisableServiceMaintenance(c.serviceID); err != nil {
			c.UI.Error(fmt.Sprintf("Error disabling service maintenance: %s", err))
			return 1
		}
		c.UI.Output(fmt.Sprintf("Service maintenance is now disabled for %q", c.serviceID))
		return 0
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Controls node or service maintenance mode"
const help = `
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

  If no arguments are given, the agent's maintenance status will be shown.
  This will return blank if nothing is currently under maintenance.
`
