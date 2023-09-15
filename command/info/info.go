// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package info

import (
	"flag"
	"fmt"
	"sort"

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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	self, err := client.Agent().Self()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying agent: %s", err))
		return 1
	}
	stats, ok := self["Stats"]
	if !ok {
		c.UI.Error(fmt.Sprintf("Agent response did not contain 'Stats' key: %v", self))
		return 1
	}

	// Get the keys in sorted order
	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Iterate over each top-level key
	for _, key := range keys {
		c.UI.Output(key + ":")

		// Sort the sub-keys
		subvals, ok := stats[key].(map[string]interface{})
		if !ok {
			c.UI.Error(fmt.Sprintf("Got invalid subkey in stats: %v", subvals))
			return 1
		}
		subkeys := make([]string, 0, len(subvals))
		for k := range subvals {
			subkeys = append(subkeys, k)
		}
		sort.Strings(subkeys)

		// Iterate over the subkeys
		for _, subkey := range subkeys {
			val := subvals[subkey]
			c.UI.Output(fmt.Sprintf("\t%s = %s", subkey, val))
		}
	}
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Provides debugging information for operators."
const help = `
Usage: consul info [options]

  Provides debugging information for operators.
`
