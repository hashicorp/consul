package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
	"net"
	"regexp"
	"strings"
)

// MembersCommand is a Command implementation that queries a running
// Consul agent what members are part of the cluster currently.
type MembersCommand struct {
	Ui cli.Ui
}

func (c *MembersCommand) Help() string {
	helpText := `
Usage: consul members [options]

  Outputs the members of a running Consul agent.

Options:

  -detailed                 Additional information such as protocol verions
                            will be shown.

  -role=<regexp>            If provided, output is filtered to only nodes matching
                            the regular expression for role

  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.

  -status=<regexp>          If provided, output is filtered to only nodes matching
                            the regular expression for status

  -wan                      If the agent is in server mode, this can be used to return
                            the other peers in the WAN pool
`
	return strings.TrimSpace(helpText)
}

func (c *MembersCommand) Run(args []string) int {
	var detailed, wan bool
	var roleFilter, statusFilter string
	cmdFlags := flag.NewFlagSet("members", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&detailed, "detailed", false, "detailed output")
	cmdFlags.BoolVar(&wan, "wan", false, "wan members")
	cmdFlags.StringVar(&roleFilter, "role", ".*", "role filter")
	cmdFlags.StringVar(&statusFilter, "status", ".*", "status filter")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Compile the regexp
	roleRe, err := regexp.Compile(roleFilter)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to compile role regexp: %v", err))
		return 1
	}
	statusRe, err := regexp.Compile(statusFilter)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to compile status regexp: %v", err))
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	defer client.Close()

	var members []agent.Member
	if wan {
		members, err = client.WANMembers()
	} else {
		members, err = client.LANMembers()
	}
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving members: %s", err))
		return 1
	}

	for _, member := range members {
		// Skip the non-matching members
		if !roleRe.MatchString(member.Tags["role"]) || !statusRe.MatchString(member.Status) {
			continue
		}

		// Format the tags as tag1=v1,tag2=v2,...
		var tagPairs []string
		for name, value := range member.Tags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", name, value))
		}
		tags := strings.Join(tagPairs, ",")

		addr := net.TCPAddr{IP: member.Addr, Port: int(member.Port)}
		c.Ui.Output(fmt.Sprintf("%s    %s    %s    %s",
			member.Name, addr.String(), member.Status, tags))

		if detailed {
			c.Ui.Output(fmt.Sprintf("    Protocol Version: %d",
				member.DelegateCur))
			c.Ui.Output(fmt.Sprintf("    Available Protocol Range: [%d, %d]",
				member.DelegateMin, member.DelegateMax))
		}
	}

	return 0
}

func (c *MembersCommand) Synopsis() string {
	return "Lists the members of a Consul cluster"
}
