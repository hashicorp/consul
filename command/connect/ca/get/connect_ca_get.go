// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package get

import (
	"encoding/json"
	"flag"
	"fmt"

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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
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
	opts := &api.QueryOptions{
		AllowStale: c.http.Stale(),
	}
	config, _, err := client.Connect().CAGetConfig(opts)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying CA configuration: %s", err))
		return 1
	}
	output, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error formatting CA configuration: %s", err))
	}
	c.UI.Output(string(output))

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Display the current Connect Certificate Authority (CA) configuration"
const help = `
Usage: consul connect ca get-config [options]

  Displays the current Connect Certificate Authority (CA) configuration.
`
