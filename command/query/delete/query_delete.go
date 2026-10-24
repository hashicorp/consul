// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delete

import (
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
	c.flags.StringVar(&c.id, "id", "", "The uuid of the prepared query to delete.")
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

	_, err = client.PreparedQuery().Delete(c.id, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting prepared query %s: %v", c.id, err))
		return 1
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Delete a prepared query"
	help     = `
Usage: consul query delete [options] -id <UUID>

  Deletes the prepared query specified by the given UUID.

  Example:

    $ consul query delete -id 182dc666-3f3f-d5ca-ec46-093dd9396ac7
`
)
