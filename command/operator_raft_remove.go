package command

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
)

type OperatorRaftRemoveCommand struct {
	BaseCommand

	// flags
	address string
	id      string
}

func (c *OperatorRaftRemoveCommand) initFlags() {
	c.InitFlagSet()
	c.FlagSet.StringVar(&c.address, "address", "",
		"The address to remove from the Raft configuration.")
	c.FlagSet.StringVar(&c.id, "id", "",
		"The ID to remove from the Raft configuration.")
}

func (c *OperatorRaftRemoveCommand) Help() string {
	c.initFlags()
	return c.HelpCommand(`
Usage: consul operator raft remove-peer [options]

Remove the Consul server with given -address from the Raft configuration.

There are rare cases where a peer may be left behind in the Raft quorum even
though the server is no longer present and known to the cluster. This command
can be used to remove the failed server so that it is no longer affects the Raft
quorum. If the server still shows in the output of the "consul members" command,
it is preferable to clean up by simply running "consul force-leave" instead of
this command.

`)
}

func (c *OperatorRaftRemoveCommand) Synopsis() string {
	return "Remove a Consul server from the Raft configuration"
}

func (c *OperatorRaftRemoveCommand) Run(args []string) int {
	c.initFlags()
	if err := c.FlagSet.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.HTTPClient()
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
