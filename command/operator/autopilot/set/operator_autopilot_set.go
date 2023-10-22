// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package set

import (
	"flag"
	"fmt"
	"strings"
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

	// flag names in cmd
	cmdFlagNames map[string]string
}

func (c *cmd) init() {
	c.cmdFlagNames = make(map[string]string)

	// Add new flags here
	c.cmdFlagNames["cleanupDeadServers"] = "cleanup-dead-servers"
	c.cmdFlagNames["maxTrailingLogs"] = "max-trailing-logs"
	c.cmdFlagNames["minQuorum"] = "min-quorum"
	c.cmdFlagNames["lastContactThreshold"] = "last-contact-threshold"
	c.cmdFlagNames["serverStabilizationTime"] = "server-stabilization-time"
	c.cmdFlagNames["redundancyZoneTag"] = "redundancy-zone-tag"
	c.cmdFlagNames["disableUpgradeMigration"] = "disable-upgrade-migration"
	c.cmdFlagNames["upgradeVersionTag"] = "upgrade-version-tag"

	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.Var(&c.cleanupDeadServers, c.cmdFlagNames["cleanupDeadServers"],
		"Controls whether Consul will automatically remove dead servers "+
			"when new ones are successfully added. Must be one of `true|false`.")
	c.flags.Var(&c.maxTrailingLogs, c.cmdFlagNames["maxTrailingLogs"],
		"Controls the maximum number of log entries that a server can trail the "+
			"leader by before being considered unhealthy.")
	c.flags.Var(&c.minQuorum, c.cmdFlagNames["minQuorum"],
		"Sets the minimum number of servers required in a cluster before autopilot "+
			"is allowed to prune dead servers.")
	c.flags.Var(&c.lastContactThreshold, c.cmdFlagNames["lastContactThreshold"],
		"Controls the maximum amount of time a server can go without contact "+
			"from the leader before being considered unhealthy. Must be a duration value "+
			"such as `200ms`.")
	c.flags.Var(&c.serverStabilizationTime, c.cmdFlagNames["serverStabilizationTime"],
		"Controls the minimum amount of time a server must be stable in the "+
			"'healthy' state before being added to the cluster. Only takes effect if all "+
			"servers are running Raft protocol version 3 or higher. Must be a duration "+
			"value such as `10s`.")
	c.flags.Var(&c.redundancyZoneTag, c.cmdFlagNames["redundancyZoneTag"],
		"(Enterprise-only) Controls the node_meta tag name used for separating servers into "+
			"different redundancy zones.")
	c.flags.Var(&c.disableUpgradeMigration, c.cmdFlagNames["disableUpgradeMigration"],
		"(Enterprise-only) Controls whether Consul will avoid promoting new servers until "+
			"it can perform a migration. Must be one of `true|false`.")
	c.flags.Var(&c.upgradeVersionTag, c.cmdFlagNames["upgradeVersionTag"],
		"(Enterprise-only) The node_meta tag to use for version info when performing upgrade "+
			"migrations. If left blank, the Consul version will be used.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	var args_1, args_2 []string

	// Ref - Issue #19266
	// converts 'arg1 = val1 arg2 = val2' -> 'arg1 val1 arg2 val2'
	// converts 'arg1= val1 arg2= val2' -> 'arg1 val1 arg2 val2'
	// converts 'arg1 =val1 arg2 =val2' -> 'arg1 val1 arg2 val2'
	for i := 0; i < len(args); i++ {
		if args[i] != "=" {
			if args[i][0] == '=' { // 'arg =val' scenario
				args_1 = append(args_1, args[i][1:])
			} else if strings.Contains(args[i], "/") { // 'arg /usr/bin/val' scenario
				args_1 = append(args_1, args[i][strings.LastIndex(args[i], "/")+1:])
			} else if args[i][len(args[i])-1] == '=' { // 'arg= val' scenario
				args_1 = append(args_1, args[i][0:len(args[i])-1])
			} else {
				args_1 = append(args_1, args[i])
			}
		}
	}
	// converts 'boolarg1 nonboolarg2 val2' -> 'boolarg1 nonboolarg2=val2'
	// converts 'arg1 val1 arg2 val2' -> 'arg1=val1 arg2=val2'
	// converts 'arg1=val1 arg2 val2' -> 'arg1=val1 arg2=val2'
	for i := 0; i < len(args_1); i++ {
		if strings.Contains(args_1[i], "=") || i == len(args_1)-1 ||
			areConsecutiveArgsFlags(c.cmdFlagNames, args_1[i], args_1[i+1]) {
			args_2 = append(args_2, args_1[i])
		} else {
			args_2 = append(args_2, args_1[i]+"="+args_1[i+1])
			i++
		}
	}

	if err := c.flags.Parse(args_2); err != nil {
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

// Checks if input cmd is like - `boolarg1 nonboolarg2 val2` where value
// for boolarg1 is not specified and its value to be assumed as true.
// Example : `cleanup-dead-servers min-quorum=5`
// Returns true if above is the case for consecutive arguments passed
func areConsecutiveArgsFlags(cmdFlagNames map[string]string, arg1 string, arg2 string) bool {
	var firstArg = arg1
	var secondArg = arg2
	var isFirstArgFlag, isSecArgFlag bool

	if arg1[0] == '-' { // -flag
		firstArg = arg1[1:]
		if firstArg[0] == '-' { // --flag
			firstArg = arg1[2:]
		}
	}

	for _, value := range cmdFlagNames {
		if firstArg == value {
			isFirstArgFlag = true
		}
		if strings.Contains(secondArg, value) {
			isSecArgFlag = true
		}
	}

	return isFirstArgFlag && isSecArgFlag
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
