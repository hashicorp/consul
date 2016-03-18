package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
	"net"
	"regexp"
	"sort"
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

  -detailed                 Provides detailed information about nodes

  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.

  -status=<regexp>          If provided, output is filtered to only nodes matching
                            the regular expression for status

  -wan                      If the agent is in server mode, this can be used to return
                            the other peers in the WAN pool
`
	return strings.TrimSpace(helpText)
}

func (c *MembersCommand) Run(args []string) int {
	var detailed bool
	var wan bool
	var statusFilter string
	cmdFlags := flag.NewFlagSet("members", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&detailed, "detailed", false, "detailed output")
	cmdFlags.BoolVar(&wan, "wan", false, "wan members")
	cmdFlags.StringVar(&statusFilter, "status", ".*", "status filter")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Compile the regexp
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

	// Filter the results
	n := len(members)
	for i := 0; i < n; i++ {
		member := members[i]
		if !statusRe.MatchString(member.Status) {
			members[i], members[n-1] = members[n-1], members[i]
			i--
			n--
			continue
		}
	}
	members = members[:n]

	// No matching members
	if len(members) == 0 {
		return 2
	}

	sort.Sort(ByMemberName(members))

	// Generate the output
	var result []string
	if detailed {
		result = c.detailedOutput(members)
	} else {
		result = c.standardOutput(members)
	}

	// Generate the columnized version
	output := columnize.SimpleFormat(result)
	c.Ui.Output(output)

	return 0
}

// so we can sort members by name
type ByMemberName []agent.Member

func (m ByMemberName) Len() int           { return len(m) }
func (m ByMemberName) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m ByMemberName) Less(i, j int) bool { return m[i].Name < m[j].Name }

// standardOutput is used to dump the most useful information about nodes
// in a more human-friendly format
func (c *MembersCommand) standardOutput(members []agent.Member) []string {
	result := make([]string, 0, len(members))
	header := "Node|Address|Status|Type|Build|Protocol|DC"
	result = append(result, header)
	for _, member := range members {
		addr := net.TCPAddr{IP: member.Addr, Port: int(member.Port)}
		protocol := member.Tags["vsn"]
		build := member.Tags["build"]
		if build == "" {
			build = "< 0.3"
		} else if idx := strings.Index(build, ":"); idx != -1 {
			build = build[:idx]
		}
		dc := member.Tags["dc"]

		switch member.Tags["role"] {
		case "node":
			line := fmt.Sprintf("%s|%s|%s|client|%s|%s|%s",
				member.Name, addr.String(), member.Status, build, protocol, dc)
			result = append(result, line)
		case "consul":
			line := fmt.Sprintf("%s|%s|%s|server|%s|%s|%s",
				member.Name, addr.String(), member.Status, build, protocol, dc)
			result = append(result, line)
		default:
			line := fmt.Sprintf("%s|%s|%s|unknown|||",
				member.Name, addr.String(), member.Status)
			result = append(result, line)
		}
	}
	return result
}

// detailedOutput is used to dump all known information about nodes in
// their raw format
func (c *MembersCommand) detailedOutput(members []agent.Member) []string {
	result := make([]string, 0, len(members))
	header := "Node|Address|Status|Tags"
	result = append(result, header)
	for _, member := range members {
		// Get the tags sorted by key
		tagKeys := make([]string, 0, len(member.Tags))
		for key := range member.Tags {
			tagKeys = append(tagKeys, key)
		}
		sort.Strings(tagKeys)

		// Format the tags as tag1=v1,tag2=v2,...
		var tagPairs []string
		for _, key := range tagKeys {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", key, member.Tags[key]))
		}

		tags := strings.Join(tagPairs, ",")

		addr := net.TCPAddr{IP: member.Addr, Port: int(member.Port)}
		line := fmt.Sprintf("%s|%s|%s|%s",
			member.Name, addr.String(), member.Status, tags)
		result = append(result, line)
	}
	return result
}

func (c *MembersCommand) Synopsis() string {
	return "Lists the members of a Consul cluster"
}
