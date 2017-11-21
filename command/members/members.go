package members

import (
	"flag"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

// cmd is a Command implementation that queries a running
// Consul agent what members are part of the cluster currently.
type cmd struct {
	UI    cli.Ui
	help  string
	flags *flag.FlagSet
	http  *flags.HTTPFlags

	// flags
	detailed     bool
	wan          bool
	statusFilter string
	segment      string
}

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.detailed, "detailed", false,
		"Provides detailed information about nodes.")
	c.flags.BoolVar(&c.wan, "wan", false,
		"If the agent is in server mode, this can be used to return the other "+
			"peers in the WAN pool.")
	c.flags.StringVar(&c.statusFilter, "status", ".*",
		"If provided, output is filtered to only nodes matching the regular "+
			"expression for status.")
	c.flags.StringVar(&c.segment, "segment", consulapi.AllSegments,
		"(Enterprise-only) If provided, output is filtered to only nodes in"+
			"the given segment.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Compile the regexp
	statusRe, err := regexp.Compile(c.statusFilter)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to compile status regexp: %v", err))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Make the request.
	opts := consulapi.MembersOpts{
		Segment: c.segment,
		WAN:     c.wan,
	}
	members, err := client.Agent().MembersOpts(opts)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error retrieving members: %s", err))
		return 1
	}

	// Filter the results
	n := len(members)
	for i := 0; i < n; i++ {
		member := members[i]
		if member.Tags["segment"] == "" {
			member.Tags["segment"] = "<default>"
		}
		if c.segment == consulapi.AllSegments && member.Tags["role"] == "consul" {
			member.Tags["segment"] = "<all>"
		}
		statusString := serf.MemberStatus(member.Status).String()
		if !statusRe.MatchString(statusString) {
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

	sort.Sort(ByMemberNameAndSegment(members))

	// Generate the output
	var result []string
	if c.detailed {
		result = c.detailedOutput(members)
	} else {
		result = c.standardOutput(members)
	}

	// Generate the columnized version
	output := columnize.SimpleFormat(result)
	c.UI.Output(output)

	return 0
}

// so we can sort members by name
type ByMemberNameAndSegment []*consulapi.AgentMember

func (m ByMemberNameAndSegment) Len() int      { return len(m) }
func (m ByMemberNameAndSegment) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m ByMemberNameAndSegment) Less(i, j int) bool {
	switch {
	case m[i].Tags["segment"] < m[j].Tags["segment"]:
		return true
	case m[i].Tags["segment"] > m[j].Tags["segment"]:
		return false
	default:
		return m[i].Name < m[j].Name
	}
}

// standardOutput is used to dump the most useful information about nodes
// in a more human-friendly format
func (c *cmd) standardOutput(members []*consulapi.AgentMember) []string {
	result := make([]string, 0, len(members))
	header := "Node|Address|Status|Type|Build|Protocol|DC|Segment"
	result = append(result, header)
	for _, member := range members {
		addr := net.TCPAddr{IP: net.ParseIP(member.Addr), Port: int(member.Port)}
		protocol := member.Tags["vsn"]
		build := member.Tags["build"]
		if build == "" {
			build = "< 0.3"
		} else if idx := strings.Index(build, ":"); idx != -1 {
			build = build[:idx]
		}
		dc := member.Tags["dc"]
		segment := member.Tags["segment"]

		statusString := serf.MemberStatus(member.Status).String()
		switch member.Tags["role"] {
		case "node":
			line := fmt.Sprintf("%s|%s|%s|client|%s|%s|%s|%s",
				member.Name, addr.String(), statusString, build, protocol, dc, segment)
			result = append(result, line)
		case "consul":
			line := fmt.Sprintf("%s|%s|%s|server|%s|%s|%s|%s",
				member.Name, addr.String(), statusString, build, protocol, dc, segment)
			result = append(result, line)
		default:
			line := fmt.Sprintf("%s|%s|%s|unknown||||",
				member.Name, addr.String(), statusString)
			result = append(result, line)
		}
	}
	return result
}

// detailedOutput is used to dump all known information about nodes in
// their raw format
func (c *cmd) detailedOutput(members []*consulapi.AgentMember) []string {
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

		addr := net.TCPAddr{IP: net.ParseIP(member.Addr), Port: int(member.Port)}
		line := fmt.Sprintf("%s|%s|%s|%s",
			member.Name, addr.String(), serf.MemberStatus(member.Status).String(), tags)
		result = append(result, line)
	}
	return result
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Lists the members of a Consul cluster"
const help = `
Usage: consul members [options]

  Outputs the members of a running Consul agent.
`
