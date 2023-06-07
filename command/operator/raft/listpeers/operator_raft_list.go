// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package listpeers

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
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
	detailed bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.detailed, "detailed", false,
		"Outputs additional information 'commit_index' which is "+
			"the index of the server's last committed Raft log entry.")
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

	result, err := raftListPeers(client, c.http.Stale())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error getting peers: %v", err))
		return 1
	}
	c.UI.Output(result)

	return 0
}

func raftListPeers(client *api.Client, stale bool) (string, error) {
	q := &api.QueryOptions{
		AllowStale: stale,
	}
	reply, err := client.Operator().RaftGetConfiguration(q)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve raft configuration: %v", err)
	}

	autoPilotReply, err := client.Operator().GetAutoPilotHealth(q)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve autopilot health: %v", err)
	}

	serverHealthDataMap := make(map[string]api.ServerHealth)
	leaderLastCommitIndex := uint64(0)

	for _, serverHealthData := range autoPilotReply.Servers {
		serverHealthDataMap[serverHealthData.ID] = serverHealthData
	}

	for _, s := range reply.Servers {
		if s.Leader {
			serverHealthDataLeader, ok := serverHealthDataMap[s.ID]
			if ok {
				leaderLastCommitIndex = serverHealthDataLeader.LastIndex
			}
		}
	}

	// Format it as a nice table.
	result := []string{"Node\x1fID\x1fAddress\x1fState\x1fVoter\x1fRaftProtocol\x1fCommit Index\x1fTrails Leader By"}
	for _, s := range reply.Servers {
		raftProtocol := s.ProtocolVersion

		if raftProtocol == "" {
			raftProtocol = "<=1"
		}
		state := "follower"
		if s.Leader {
			state = "leader"
		}

		serverHealthData, ok := serverHealthDataMap[s.ID]
		if ok {
			trailsLeaderBy := leaderLastCommitIndex - serverHealthData.LastIndex
			trailsLeaderByText := fmt.Sprintf("%d Commits", trailsLeaderBy)
			if s.Leader {
				trailsLeaderByText = "_"
			} else if trailsLeaderBy <= 1 {
				trailsLeaderByText = fmt.Sprintf("%d Commit", trailsLeaderBy)
			}
			result = append(result, fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%v\x1f%s\x1f%v\x1f%s",
				s.Node, s.ID, s.Address, state, s.Voter, raftProtocol, serverHealthData.LastIndex, trailsLeaderByText))
		} else {
			result = append(result, fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%v\x1f%s\x1f%v",
				s.Node, s.ID, s.Address, state, s.Voter, raftProtocol, "_"))
		}
	}

	return columnize.Format(result, &columnize.Config{Delim: string([]byte{0x1f})}), nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Display the current Raft peer configuration"
const help = `
Usage: consul operator raft list-peers [options]

  Displays the current Raft peer configuration.
`
