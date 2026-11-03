// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package execute

import (
	"encoding/json"
	"flag"
	"fmt"

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

	id string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.id, "id", "", "The uuid of the prepared query to execute.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.id == "" {
		c.UI.Error("Must specify the -id parameter")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	result, _, err := client.PreparedQuery().Execute(c.id, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error executing prepared query %s: %v", c.id, err))
		return 1
	}

	b, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		c.UI.Error("Failed to encode output data")
		return 1
	}

	c.UI.Info(string(b))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Execute a prepared query"
	help     = `
Usage: consul query execute [options] -id <UUID>

  Executes the prepared query specified by the given UUID.

  Example:

    $ consul query execute -id 182dc666-3f3f-d5ca-ec46-093dd9396ac7
`
)
