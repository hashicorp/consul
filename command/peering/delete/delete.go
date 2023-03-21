// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package delete

import (
	"context"
	"flag"
	"fmt"

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

	name string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.name, "name", "", "(Required) The local name assigned to the peer cluster.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.name == "" {
		c.UI.Error("Missing the required -name flag")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	peerings := client.Peerings()

	_, err = peerings.Delete(context.Background(), c.name, &api.WriteOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting peering for %s: %v", c.name, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Successfully submitted peering connection, %s, for deletion", c.name))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Delete a peering connection"
	help     = `
Usage: consul peering delete [options] -name <peer name>

  Delete a peering connection.  Consul deletes all data imported from the peer 
  in the background. The peering connection is removed after all associated 
  data has been deleted. Operators can still read the peering connections 
  while the data is being removed. A 'DeletedAt' field will be populated with 
  the timestamp of when the peering was marked for deletion.

  Example:

    $ consul peering delete -name west-dc
`
)
