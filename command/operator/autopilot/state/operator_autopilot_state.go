// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
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

	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(
		&c.format,
		"format",
		PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(GetSupportedFormats(), "|")),
	)
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
	state, err := client.Operator().AutopilotState(opts)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Autopilot state: %s", err))
		return 1
	}

	formatter, err := NewFormatter(c.format)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	out, err := formatter.FormatState(state)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if out != "" {
		c.UI.Info(out)
	}
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Display the current Autopilot configuration"
const help = `
Usage: consul operator autopilot get-config [options]

  Displays the current Autopilot configuration.
`
