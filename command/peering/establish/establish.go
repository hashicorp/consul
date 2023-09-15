// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package establish

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

	name         string
	peeringToken string
	meta         map[string]string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.name, "name", "", "(Required) The local name assigned to the peer cluster.")

	c.flags.StringVar(&c.peeringToken, "peering-token", "", "(Required) The peering token from the accepting cluster.")

	c.flags.Var((*flags.FlagMapValue)(&c.meta), "meta",
		"Metadata to associate with the peering, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple meta fields.")

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

	if c.peeringToken == "" {
		c.UI.Error("Missing the required -peering-token flag")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	peerings := client.Peerings()

	req := api.PeeringEstablishRequest{
		PeerName:     c.name,
		PeeringToken: c.peeringToken,
		Partition:    c.http.Partition(),
		Meta:         c.meta,
	}

	_, _, err = peerings.Establish(context.Background(), req, &api.WriteOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error establishing peering for %s: %v", req.PeerName, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Successfully established peering connection with %s", req.PeerName))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Establish a peering connection"
	help     = `
Usage: consul peering establish [options] -name <peer name> -peering-token <token>

  Establish a peering connection. The name provided will be used locally by
  this cluster to refer to the peering connection. The peering token can 
  only be used once to establish the connection.

  Example:

    $ consul peering establish -name west-dc -peering-token <token>
`
)
