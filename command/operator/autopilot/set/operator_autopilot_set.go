// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package set

import (
	"flag"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
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
	help  string

	// flags
	cleanupDeadServers      flags.BoolValue
	maxTrailingLogs         flags.UintValue
	minQuorum               flags.UintValue
	lastContactThreshold    flags.DurationValue
	serverStabilizationTime flags.DurationValue
	redundancyZoneTag       flags.StringValue
	disableUpgradeMigration flags.BoolValue
	upgradeVersionTag       flags.StringValue
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.Var(&c.cleanupDeadServers, "cleanup-dead-servers",
		"Controls whether Consul will automatically remove dead servers "+
			"when new ones are successfully added. Must be one of `true|false`.")
	c.flags.Var(&c.maxTrailingLogs, "max-trailing-logs",
		"Controls the maximum number of log entries that a server can trail the "+
			"leader by before being considered unhealthy.")
	c.flags.Var(&c.minQuorum, "min-quorum",
		"Sets the minimum number of servers required in a cluster before autopilot "+
			"is allowed to prune dead servers.")
	c.flags.Var(&c.lastContactThreshold, "last-contact-threshold",
		"Controls the maximum amount of time a server can go without contact "+
			"from the leader before being considered unhealthy. Must be a duration value "+
			"such as `200ms`.")
	c.flags.Var(&c.serverStabilizationTime, "server-stabilization-time",
		"Controls the minimum amount of time a server must be stable in the "+
			"'healthy' state before being added to the cluster. Only takes effect if all "+
			"servers are running Raft protocol version 3 or higher. Must be a duration "+
			"value such as `10s`.")
	c.flags.Var(&c.redundancyZoneTag, "redundancy-zone-tag",
		"(Enterprise-only) Controls the node_meta tag name used for separating servers into "+
			"different redundancy zones.")
	c.flags.Var(&c.disableUpgradeMigration, "disable-upgrade-migration",
		"(Enterprise-only) Controls whether Consul will avoid promoting new servers until "+
			"it can perform a migration. Must be one of `true|false`.")
	c.flags.Var(&c.upgradeVersionTag, "upgrade-version-tag",
		"(Enterprise-only) The node_meta tag to use for version info when performing upgrade "+
			"migrations. If left blank, the Consul version will be used.")

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
	c.minQuorum.Merge(&conf.MinQuorum)

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

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Modify the current Autopilot configuration"
const help = `
Usage: consul operator autopilot set-config [options]

  Modifies the current Autopilot configuration.
`
