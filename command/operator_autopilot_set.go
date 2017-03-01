package command

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/command/base"
)

type OperatorAutopilotSetCommand struct {
	base.Command
}

func (c *OperatorAutopilotSetCommand) Help() string {
	helpText := `
Usage: consul operator autopilot set-config [options]

Modifies the current Autopilot configuration.

` + c.Command.Help()

	return strings.TrimSpace(helpText)
}

func (c *OperatorAutopilotSetCommand) Synopsis() string {
	return "Modify the current Autopilot configuration"
}

func (c *OperatorAutopilotSetCommand) Run(args []string) int {
	var cleanupDeadServers base.BoolValue
	var lastContactThresholdRaw string
	var maxTrailingLogs base.UintValue
	var serverStabilizationTimeRaw string

	f := c.Command.NewFlagSet(c)

	f.Var(&cleanupDeadServers, "cleanup-dead-servers",
		"Controls whether Consul will automatically remove dead servers "+
			"when new ones are successfully added. Must be one of `true|false`.")
	f.Var(&maxTrailingLogs, "max-trailing-logs",
		"Controls the maximum number of log entries that a server can trail the "+
			"leader by before being considered unhealthy.")
	f.StringVar(&lastContactThresholdRaw, "last-contact-threshold", "",
		"Controls the maximum amount of time a server can go without contact "+
			"from the leader before being considered unhealthy. Must be a duration value "+
			"such as `10s`.")
	f.StringVar(&serverStabilizationTimeRaw, "server-stabilization-time", "",
		"Controls the minimum amount of time a server must be stable in the "+
			"'healthy' state before being added to the cluster. Only takes effect if all "+
			"servers are running Raft protocol version 3 or higher. Must be a duration "+
			"value such as `10s`.")

	if err := c.Command.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.Command.HTTPClient()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch the current configuration.
	operator := client.Operator()
	conf, err := operator.AutopilotGetConfiguration(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Autopilot configuration: %s", err))
		return 1
	}

	// Update the config values based on the set flags.
	cleanupDeadServers.Merge(&conf.CleanupDeadServers)
	trailing := uint(conf.MaxTrailingLogs)
	maxTrailingLogs.Merge(&trailing)
	conf.MaxTrailingLogs = uint64(trailing)

	if lastContactThresholdRaw != "" {
		dur, err := time.ParseDuration(lastContactThresholdRaw)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("invalid value for last-contact-threshold: %v", err))
			return 1
		}
		conf.LastContactThreshold = dur
	}
	if serverStabilizationTimeRaw != "" {
		dur, err := time.ParseDuration(serverStabilizationTimeRaw)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("invalid value for server-stabilization-time: %v", err))
		}
		conf.ServerStabilizationTime = dur
	}

	// Check-and-set the new configuration.
	result, err := operator.AutopilotCASConfiguration(conf, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error setting Autopilot configuration: %s", err))
		return 1
	}
	if result {
		c.Ui.Output("Configuration updated!")
		return 0
	} else {
		c.Ui.Output("Configuration could not be atomically updated, please try again")
		return 1
	}
}
