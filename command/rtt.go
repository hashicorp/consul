package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/serf/coordinate"
	"github.com/mitchellh/cli"
)

// RTTCommand is a Command implementation that allows users to query the
// estimated round trip time between nodes using network coordinates.
type RTTCommand struct {
	Ui cli.Ui
}

func (c *RTTCommand) Help() string {
	helpText := `
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
  because they are maintained by independent Serf gossip pools, so they are
  not compatible.

Options:

  -wan                       Use WAN coordinates instead of LAN coordinates.
  -http-addr=127.0.0.1:8500  HTTP address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *RTTCommand) Run(args []string) int {
	var wan bool

	cmdFlags := flag.NewFlagSet("rtt", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.BoolVar(&wan, "wan", false, "wan")
	httpAddr := HTTPAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// They must provide at least one node.
	nodes := cmdFlags.Args()
	if len(nodes) < 1 || len(nodes) > 2 {
		c.Ui.Error("One or two node names must be specified")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	// Create and test the HTTP client.
	conf := api.DefaultConfig()
	conf.Address = *httpAddr
	client, err := api.NewClient(conf)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	coordClient := client.Coordinate()

	var source string
	var coord1, coord2 *coordinate.Coordinate
	if wan {
		source = "WAN"

		// Default the second node to the agent if none was given.
		if len(nodes) < 2 {
			agent := client.Agent()
			self, err := agent.Self()
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Unable to look up agent info: %s", err))
				return 1
			}

			node, dc := self["Config"]["NodeName"], self["Config"]["Datacenter"]
			nodes = append(nodes, fmt.Sprintf("%s.%s", node, dc))
		}

		// Parse the input nodes.
		parts1 := strings.Split(nodes[0], ".")
		parts2 := strings.Split(nodes[1], ".")
		if len(parts1) != 2 || len(parts2) != 2 {
			c.Ui.Error("Node names must be specified as <datacenter>.<node name> with -wan")
			return 1
		}
		node1, dc1 := parts1[0], parts1[1]
		node2, dc2 := parts2[0], parts2[1]

		// Pull all the WAN coordinates.
		dcs, err := coordClient.Datacenters()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error getting coordinates: %s", err))
			return 1
		}

		// See if the requested nodes are in there.
		for _, dc := range dcs {
			for _, entry := range dc.Coordinates {
				if dc.Datacenter == dc1 && entry.Node == node1 {
					coord1 = entry.Coord
				}
				if dc.Datacenter == dc2 && entry.Node == node2 {
					coord2 = entry.Coord
				}

				if coord1 != nil && coord2 != nil {
					goto SHOW_RTT
				}
			}
		}
	} else {
		source = "LAN"

		// Default the second node to the agent if none was given.
		if len(nodes) < 2 {
			agent := client.Agent()
			node, err := agent.NodeName()
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Unable to look up agent info: %s", err))
				return 1
			}
			nodes = append(nodes, node)
		}

		// Pull all the LAN coordinates.
		entries, _, err := coordClient.Nodes(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error getting coordinates: %s", err))
			return 1
		}

		// See if the requested nodes are in there.
		for _, entry := range entries {
			if entry.Node == nodes[0] {
				coord1 = entry.Coord
			}
			if entry.Node == nodes[1] {
				coord2 = entry.Coord
			}

			if coord1 != nil && coord2 != nil {
				goto SHOW_RTT
			}
		}
	}

	// Make sure we found both coordinates.
	if coord1 == nil {
		c.Ui.Error(fmt.Sprintf("Could not find a coordinate for node %q", nodes[0]))
		return 1
	}
	if coord2 == nil {
		c.Ui.Error(fmt.Sprintf("Could not find a coordinate for node %q", nodes[1]))
		return 1
	}

SHOW_RTT:

	// Report the round trip time.
	dist := fmt.Sprintf("%.3f ms", coord1.DistanceTo(coord2).Seconds()*1000.0)
	c.Ui.Output(fmt.Sprintf("Estimated %s <-> %s rtt: %s (using %s coordinates)", nodes[0], nodes[1], dist, source))
	return 0
}

func (c *RTTCommand) Synopsis() string {
	return "Estimates network round trip time between nodes"
}
