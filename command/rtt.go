package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/serf/coordinate"
	"github.com/mitchellh/cli"
)

// RttCommand is a Command implementation that allows users to query the
// estimated round trip time between nodes using network coordinates.
type RttCommand struct {
	Ui cli.Ui
}

func (c *RttCommand) Help() string {
	helpText := `
Usage: consul rtt [options] node1 node2

  Estimates the round trip time between two nodes using Consul's network
  coordinate model of the cluster.

  By default, the two nodes are assumed to be nodes in the local datacenter
  and the LAN coordinates are used. If the -wan option is given, then the WAN
  coordinates are used, and the node names must be prefixed by the datacenter
  and a period (eg. "dc1.sever").

  It is not possible to measure between LAN coordinates and WAN coordinates
  because they are maintained by independent Serf gossip pools, so they are
  not compatible.

  The two node names are required. Note that these are node names as known to
  Consul as "consul members" would show, not IP addresses.

Options:

  -wan                       Use WAN coordinates instead of LAN coordinates.
  -http-addr=127.0.0.1:8500  HTTP address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *RttCommand) Run(args []string) int {
	var wan bool

	cmdFlags := flag.NewFlagSet("rtt", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.BoolVar(&wan, "wan", false, "wan")
	httpAddr := HTTPAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// They must provide a pair of nodes.
	nodes := cmdFlags.Args()
	if len(nodes) != 2 {
		c.Ui.Error("Two node names must be specified")
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
		// Parse the input nodes.
		parts1 := strings.Split(nodes[0], ".")
		parts2 := strings.Split(nodes[1], ".")
		if len(parts1) != 2 || len(parts2) != 2 {
			c.Ui.Error("Node names must be specified as <datacenter>.<node name> with -wan")
			return 1
		}
		dc1, node1 := parts1[0], parts1[1]
		dc2, node2 := parts2[0], parts2[1]

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
			}
		}
		source = "WAN"
	} else {
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
		}
		source = "LAN"
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

	// Report the round trip time.
	dist := coord1.DistanceTo(coord2).Seconds()
	c.Ui.Output(fmt.Sprintf("Estimated %s <-> %s rtt=%.3f ms (using %s coordinates)", nodes[0], nodes[1], dist*1000.0, source))
	return 0
}

func (c *RttCommand) Synopsis() string {
	return "Estimates round trip times between nodes"
}
