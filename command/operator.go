package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

// OperatorCommand is used to provide various low-level tools for Consul
// operators.
type OperatorCommand struct {
	Ui cli.Ui
}

func (c *OperatorCommand) Help() string {
	helpText := `
Usage: consul operator <subcommand> [common options] [action] [options]

  Provides cluster-level tools for Consul operators, such as interacting with
  the Raft subsystem. NOTE: Use this command with extreme caution, as improper
  use could lead to a Consul outage and even loss of data.

  If ACLs are enabled then a token with operator privileges may be required in
  order to use this command. Requests are forwarded internally to the leader
  if required, so this can be run from any Consul node in a cluster.

  Run consul operator <subcommand> with no arguments for help on that
  subcommand.

Common Options:

  -http-addr=127.0.0.1:8500  HTTP address of the Consul agent.
  -token=""                  ACL token to use. Defaults to that of agent.

Subcommands:

  raft                       View and modify Consul's Raft configuration.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorCommand) Run(args []string) int {
	if len(args) < 1 {
		c.Ui.Error("A subcommand must be specified")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	var err error
	subcommand := args[0]
	switch subcommand {
	case "raft":
		err = c.raft(args[1:])
	default:
		err = fmt.Errorf("unknown subcommand %q", subcommand)
	}

	if err != nil {
		c.Ui.Error(fmt.Sprintf("Operator %q subcommand failed: %v", subcommand, err))
		return 1
	}
	return 0
}

// Synopsis returns a one-line description of this command.
func (c *OperatorCommand) Synopsis() string {
	return "Provides cluster-level tools for Consul operators"
}

const raftHelp = `
Raft Subcommand Actions:

  raft -list-peers -stale=[true|false]

     Displays the current Raft peer configuration.

     The -stale argument defaults to "false" which means the leader provides the
     result. If the cluster is in an outage state without a leader, you may need
     to set -stale to "true" to get the configuration from a non-leader server.

  raft -remove-peer -address="IP:port"

     Removes Consul server with given -address from the Raft configuration.

     There are rare cases where a peer may be left behind in the Raft quorum even
     though the server is no longer present and known to the cluster. This
     command can be used to remove the failed server so that it is no longer
     affects the Raft quorum. If the server still shows in the output of the
     "consul members" command, it is preferable to clean up by simply running
     "consul force-leave" instead of this command.
`

// raft handles the raft subcommands.
func (c *OperatorCommand) raft(args []string) error {
	cmdFlags := flag.NewFlagSet("raft", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	// Parse verb arguments.
	var listPeers, removePeer bool
	cmdFlags.BoolVar(&listPeers, "list-peers", false, "")
	cmdFlags.BoolVar(&removePeer, "remove-peer", false, "")

	// Parse other arguments.
	var stale bool
	var address, token string
	cmdFlags.StringVar(&address, "address", "", "")
	cmdFlags.BoolVar(&stale, "stale", false, "")
	cmdFlags.StringVar(&token, "token", "", "")
	httpAddr := HTTPAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return err
	}

	// Set up a client.
	conf := api.DefaultConfig()
	conf.Address = *httpAddr
	client, err := api.NewClient(conf)
	if err != nil {
		return fmt.Errorf("error connecting to Consul agent: %s", err)
	}
	operator := client.Operator()

	// Dispatch based on the verb argument.
	if listPeers {
		// Fetch the current configuration.
		q := &api.QueryOptions{
			AllowStale: stale,
			Token:      token,
		}
		reply, err := operator.RaftGetConfiguration(q)
		if err != nil {
			return err
		}

		// Format it as a nice table.
		result := []string{"Node|ID|Address|State|Voter"}
		for _, s := range reply.Servers {
			state := "follower"
			if s.Leader {
				state = "leader"
			}
			result = append(result, fmt.Sprintf("%s|%s|%s|%s|%v",
				s.Node, s.ID, s.Address, state, s.Voter))
		}
		c.Ui.Output(columnize.SimpleFormat(result))
	} else if removePeer {
		// TODO (slackpad) Once we expose IDs, add support for removing
		// by ID, add support for that.
		if len(address) == 0 {
			return fmt.Errorf("an address is required for the peer to remove")
		}

		// Try to kick the peer.
		w := &api.WriteOptions{
			Token: token,
		}
		if err := operator.RaftRemovePeerByAddress(address, w); err != nil {
			return err
		}
		c.Ui.Output(fmt.Sprintf("Removed peer with address %q", address))
	} else {
		c.Ui.Output(c.Help())
		c.Ui.Output("")
		c.Ui.Output(strings.TrimSpace(raftHelp))
	}

	return nil
}
