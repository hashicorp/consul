// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package removepeer

import (
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

	// flags
	address string
	id      string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.address, "address", "",
		"The address to remove from the Raft configuration.")
	c.flags.StringVar(&c.id, "id", "",
		"The ID to remove from the Raft configuration.")

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
	if err := raftRemovePeers(c.address, c.id, client.Operator()); err != nil {
		c.UI.Error(fmt.Sprintf("Error removing peer: %v", err))
		return 1
	}
	if c.address != "" {
		c.UI.Output(fmt.Sprintf("Removed peer with address %q", c.address))
	} else {
		c.UI.Output(fmt.Sprintf("Removed peer with id %q", c.id))
	}

	return 0
}

func raftRemovePeers(address, id string, operator *api.Operator) error {
	if len(address) == 0 && len(id) == 0 {
		return fmt.Errorf("an address or id is required for the peer to remove")
	}
	if len(address) > 0 && len(id) > 0 {
		return fmt.Errorf("cannot give both an address and id")
	}

	// Try to kick the peer.
	if len(address) > 0 {
		if err := operator.RaftRemovePeerByAddress(address, nil); err != nil {
			return err
		}
	} else {
		if err := operator.RaftRemovePeerByID(id, nil); err != nil {
			return err
		}
	}

	return nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Remove a Consul server from the Raft configuration"
const help = `
Usage: consul operator raft remove-peer [options]

  Remove the Consul server with given -address from the Raft configuration.

  There are rare cases where a peer may be left behind in the Raft quorum even
  though the server is no longer present and known to the cluster. This command
  can be used to remove the failed server so that it is no longer affects the Raft
  quorum. If the server still shows in the output of the "consul members" command,
  it is preferable to clean up by simply running "consul force-leave" instead of
  this command.
`
