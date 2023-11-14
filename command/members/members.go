// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package members

import (
	"flag"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"

	"github.com/hashicorp/consul/acl"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
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
	filter       string
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
	c.flags.StringVar(&c.filter, "filter", "", "Filter to use with the request")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
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
		Filter:  c.filter,
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
		if member.Tags[consulapi.MemberTagKeyPartition] == "" {
			member.Tags[consulapi.MemberTagKeyPartition] = "default"
		}
		if acl.IsDefaultPartition(member.Tags[consulapi.MemberTagKeyPartition]) {
			if c.segment == consulapi.AllSegments && member.Tags[consulapi.MemberTagKeyRole] == consulapi.MemberTagValueRoleServer {
				member.Tags[consulapi.MemberTagKeySegment] = "<all>"
			} else if member.Tags[consulapi.MemberTagKeySegment] == "" {
				member.Tags[consulapi.MemberTagKeySegment] = "<default>"
			}
		} else {
			member.Tags[consulapi.MemberTagKeySegment] = ""
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

	sort.Sort(ByMemberNamePartitionAndSegment(members))

	// Generate the output
	var result []string
	if c.detailed {
		result = c.detailedOutput(members)
	} else {
		result = c.standardOutput(members)
	}

	// Generate the columnized version
	output := columnize.Format(result, &columnize.Config{Delim: string([]byte{0x1f})})
	c.UI.Output(output)

	return 0
}

// ByMemberNamePartitionAndSegment sorts members by name with a stable sort.
//
// 1. servers go at the top
// 2. members of the default partition go next (including segments)
// 3. members of partitions follow
type ByMemberNamePartitionAndSegment []*consulapi.AgentMember

func (m ByMemberNamePartitionAndSegment) Len() int      { return len(m) }
func (m ByMemberNamePartitionAndSegment) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m ByMemberNamePartitionAndSegment) Less(i, j int) bool {
	tags_i := parseTags(m[i].Tags)
	tags_j := parseTags(m[j].Tags)

	// put role=consul first
	switch {
	case tags_i.role == consulapi.MemberTagValueRoleServer && tags_j.role != consulapi.MemberTagValueRoleServer:
		return true
	case tags_i.role != consulapi.MemberTagValueRoleServer && tags_j.role == consulapi.MemberTagValueRoleServer:
		return false
	}

	// then the default partitions
	switch {
	case isDefault(tags_i.partition) && !isDefault(tags_j.partition):
		return true
	case !isDefault(tags_i.partition) && isDefault(tags_j.partition):
		return false
	}

	// then by segments within the default
	switch {
	case tags_i.segment < tags_j.segment:
		return true
	case tags_i.segment > tags_j.segment:
		return false
	}

	// then by partitions
	switch {
	case tags_i.partition < tags_j.partition:
		return true
	case tags_i.partition > tags_j.partition:
		return false
	}

	// finally by name
	return m[i].Name < m[j].Name
}

func isDefault(s string) bool {
	// NOTE: we can't use structs.IsDefaultPartition since that discards the input
	return s == "" || s == "default"
}

// standardOutput is used to dump the most useful information about nodes
// in a more human-friendly format
func (c *cmd) standardOutput(members []*consulapi.AgentMember) []string {
	result := make([]string, 0, len(members))
	header := "Node\x1fAddress\x1fStatus\x1fType\x1fBuild\x1fProtocol\x1fDC\x1fPartition\x1fSegment"
	result = append(result, header)
	for _, member := range members {
		tags := parseTags(member.Tags)

		addr := net.TCPAddr{IP: net.ParseIP(member.Addr), Port: int(member.Port)}
		protocol := member.Tags["vsn"]
		build := member.Tags["build"]
		if build == "" {
			build = "< 0.3"
		} else if idx := strings.Index(build, ":"); idx != -1 {
			build = build[:idx]
		}

		statusString := serf.MemberStatus(member.Status).String()
		switch tags.role {
		case consulapi.MemberTagValueRoleClient:
			line := fmt.Sprintf("%s\x1f%s\x1f%s\x1fclient\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s",
				member.Name, addr.String(), statusString, build, protocol, tags.datacenter, tags.partition, tags.segment)
			result = append(result, line)

		case consulapi.MemberTagValueRoleServer:
			line := fmt.Sprintf("%s\x1f%s\x1f%s\x1fserver\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s",
				member.Name, addr.String(), statusString, build, protocol, tags.datacenter, tags.partition, tags.segment)
			result = append(result, line)

		default:
			line := fmt.Sprintf("%s\x1f%s\x1f%s\x1funknown\x1f\x1f\x1f\x1f\x1f",
				member.Name, addr.String(), statusString)
			result = append(result, line)
		}
	}
	return result
}

type decodedTags struct {
	role       string
	segment    string
	partition  string
	datacenter string
}

func parseTags(tags map[string]string) decodedTags {
	return decodedTags{
		role:       tags[consulapi.MemberTagKeyRole],
		segment:    tags[consulapi.MemberTagKeySegment],
		partition:  tags[consulapi.MemberTagKeyPartition],
		datacenter: tags[consulapi.MemberTagKeyDatacenter],
	}
}

// detailedOutput is used to dump all known information about nodes in
// their raw format
func (c *cmd) detailedOutput(members []*consulapi.AgentMember) []string {
	result := make([]string, 0, len(members))
	header := "Node\x1fAddress\x1fStatus\x1fTags"
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
		line := fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s",
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
