// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rtt

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/serf/coordinate"
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/lib"
)

// TODO(partitions): how will this command work when asking for RTT between a
// partitioned client and a server in the default partition?

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
	wan bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.wan, "wan", false,
		"Use WAN coordinates instead of LAN coordinates.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// They must provide at least one node.
	nodes := c.flags.Args()
	if len(nodes) < 1 || len(nodes) > 2 {
		c.UI.Error("One or two node names must be specified")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	// Create and test the HTTP client.
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	coordClient := client.Coordinate()

	var source string
	var coord1, coord2 *coordinate.Coordinate
	if c.wan {
		source = "WAN"

		// Default the second node to the agent if none was given.
		if len(nodes) < 2 {
			agent := client.Agent()
			self, err := agent.Self()
			if err != nil {
				c.UI.Error(fmt.Sprintf("Unable to look up agent info: %s", err))
				return 1
			}

			node, dc := self["Config"]["NodeName"], self["Config"]["Datacenter"]
			nodes = append(nodes, fmt.Sprintf("%s.%s", node, dc))
		}

		// Parse the input nodes.
		parts1 := strings.Split(nodes[0], ".")
		parts2 := strings.Split(nodes[1], ".")
		if len(parts1) != 2 || len(parts2) != 2 {
			c.UI.Error("Node names must be specified as <node name>.<datacenter> with -wan")
			return 1
		}
		node1, dc1 := parts1[0], parts1[1]
		node2, dc2 := parts2[0], parts2[1]

		// Pull all the WAN coordinates.
		dcs, err := coordClient.Datacenters()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error getting coordinates: %s", err))
			return 1
		}

		// See if the requested nodes are in there. We only compare
		// coordinates in the same areas.
		var area1, area2 string
		for _, dc := range dcs {
			for _, entry := range dc.Coordinates {
				if dc.Datacenter == dc1 && strings.EqualFold(entry.Node, node1) {
					area1 = dc.AreaID
					coord1 = entry.Coord
				}
				if dc.Datacenter == dc2 && strings.EqualFold(entry.Node, node2) {
					area2 = dc.AreaID
					coord2 = entry.Coord
				}

				if area1 == area2 && coord1 != nil && coord2 != nil {
					goto SHOW_RTT
				}
			}
		}

		// Nil out the coordinates so we don't display across areas if
		// we didn't find anything.
		coord1, coord2 = nil, nil
	} else {
		source = "LAN"

		// Default the second node to the agent if none was given.
		if len(nodes) < 2 {
			agent := client.Agent()
			node, err := agent.NodeName()
			if err != nil {
				c.UI.Error(fmt.Sprintf("Unable to look up agent info: %s", err))
				return 1
			}
			nodes = append(nodes, node)
		}

		// Pull all the LAN coordinates.
		entries, _, err := coordClient.Nodes(nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error getting coordinates: %s", err))
			return 1
		}

		// Index all the coordinates by segment.
		cs1, cs2 := make(lib.CoordinateSet), make(lib.CoordinateSet)
		for _, entry := range entries {
			if strings.EqualFold(entry.Node, nodes[0]) {
				cs1[entry.Segment] = entry.Coord
			}
			if strings.EqualFold(entry.Node, nodes[1]) {
				cs2[entry.Segment] = entry.Coord
			}
		}

		// See if there's a compatible set of coordinates.
		coord1, coord2 = cs1.Intersect(cs2)
		if coord1 != nil && coord2 != nil {
			goto SHOW_RTT
		}
	}

	// Make sure we found both coordinates.
	if coord1 == nil {
		c.UI.Error(fmt.Sprintf("Could not find a coordinate for node %q", nodes[0]))
		return 1
	}
	if coord2 == nil {
		c.UI.Error(fmt.Sprintf("Could not find a coordinate for node %q", nodes[1]))
		return 1
	}

SHOW_RTT:

	// Report the round trip time.
	dist := fmt.Sprintf("%.3f ms", coord1.DistanceTo(coord2).Seconds()*1000.0)
	c.UI.Output(fmt.Sprintf("Estimated %s <-> %s rtt: %s (using %s coordinates)", nodes[0], nodes[1], dist, source))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Estimates network round trip time between nodes"
const help = `
Usage: consul rtt [options] node1 [node2]

  Estimates the round trip time between two nodes using Consul's network
  coordinate model of the cluster.

  At least one node name is required. If the second node name isn't given, it
  is set to the agent's node name. Note that these are node names as known to
  Consul as "consul members" would show, not IP addresses.

  By default, the two nodes are assumed to be nodes in the local datacenter
  and the LAN coordinates are used. If the -wan option is given, then the WAN
  coordinates are used, and the node names must be suffixed by a period and
  the datacenter (eg. "myserver.dc1").

  It is not possible to measure between LAN coordinates and WAN coordinates
  because they are maintained by independent Serf gossip areas, so they are
  not compatible.
`
