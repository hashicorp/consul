package command

import (
	"flag"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/configutil"
)

type OperatorAutopilotSetCommand struct {
	BaseCommand

	// flags
	cleanupDeadServers      configutil.BoolValue
	maxTrailingLogs         configutil.UintValue
	lastContactThreshold    configutil.DurationValue
	serverStabilizationTime configutil.DurationValue
	redundancyZoneTag       configutil.StringValue
	disableUpgradeMigration configutil.BoolValue
	upgradeVersionTag       configutil.StringValue
}

func (c *OperatorAutopilotSetCommand) initFlags() {
	c.InitFlagSet()
	c.FlagSet.Var(&c.cleanupDeadServers, "cleanup-dead-servers",
		"Controls whether Consul will automatically remove dead servers "+
			"when new ones are successfully added. Must be one of `true|false`.")
	c.FlagSet.Var(&c.maxTrailingLogs, "max-trailing-logs",
		"Controls the maximum number of log entries that a server can trail the "+
			"leader by before being considered unhealthy.")
	c.FlagSet.Var(&c.lastContactThreshold, "last-contact-threshold",
		"Controls the maximum amount of time a server can go without contact "+
			"from the leader before being considered unhealthy. Must be a duration value "+
			"such as `200ms`.")
	c.FlagSet.Var(&c.serverStabilizationTime, "server-stabilization-time",
		"Controls the minimum amount of time a server must be stable in the "+
			"'healthy' state before being added to the cluster. Only takes effect if all "+
			"servers are running Raft protocol version 3 or higher. Must be a duration "+
			"value such as `10s`.")
	c.FlagSet.Var(&c.redundancyZoneTag, "redundancy-zone-tag",
		"(Enterprise-only) Controls the node_meta tag name used for separating servers into "+
			"different redundancy zones.")
	c.FlagSet.Var(&c.disableUpgradeMigration, "disable-upgrade-migration",
		"(Enterprise-only) Controls whether Consul will avoid promoting new servers until "+
			"it can perform a migration. Must be one of `true|false`.")
	c.FlagSet.Var(&c.upgradeVersionTag, "upgrade-version-tag",
		"(Enterprise-only) The node_meta tag to use for version info when performing upgrade "+
			"migrations. If left blank, the Consul version will be used.")
}

func (c *OperatorAutopilotSetCommand) Help() string {
	c.initFlags()
	return c.HelpCommand(`
Usage: consul operator autopilot set-config [options]

Modifies the current Autopilot configuration.

`)
}

func (c *OperatorAutopilotSetCommand) Synopsis() string {
	return "Modify the current Autopilot configuration"
}

func (c *OperatorAutopilotSetCommand) Run(args []string) int {
	c.initFlags()
	if err := c.FlagSet.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch the current configuration.
	operator := client.Operator()
	conf, err := operator.AutopilotGetConfiguration(nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Autopilot configuration: %s", err))
		return 1
	}

	// Update the config values based on the set flags.
	c.cleanupDeadServers.Merge(&conf.CleanupDeadServers)
	c.redundancyZoneTag.Merge(&conf.RedundancyZoneTag)
	c.disableUpgradeMigration.Merge(&conf.DisableUpgradeMigration)
	c.upgradeVersionTag.Merge(&conf.UpgradeVersionTag)

	trailing := uint(conf.MaxTrailingLogs)
	c.maxTrailingLogs.Merge(&trailing)
	conf.MaxTrailingLogs = uint64(trailing)

	last := time.Duration(*conf.LastContactThreshold)
	c.lastContactThreshold.Merge(&last)
	conf.LastContactThreshold = api.NewReadableDuration(last)

	stablization := time.Duration(*conf.ServerStabilizationTime)
	c.serverStabilizationTime.Merge(&stablization)
	conf.ServerStabilizationTime = api.NewReadableDuration(stablization)

	// Check-and-set the new configuration.
	result, err := operator.AutopilotCASConfiguration(conf, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error setting Autopilot configuration: %s", err))
		return 1
	}
	if result {
		c.UI.Output("Configuration updated!")
		return 0
	}
	c.UI.Output("Configuration could not be atomically updated, please try again")
	return 1
}
