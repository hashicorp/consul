package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/configutil"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

var _ cli.Command = (*CatalogListNodesCommand)(nil)

// CatalogListNodesCommand is a Command implementation that is used to fetch all the
// nodes in the catalog.
type CatalogListNodesCommand struct {
	BaseCommand
}

func (c *CatalogListNodesCommand) Help() string {
	helpText := `
Usage: consul catalog nodes [options]

  Retrieves the list nodes registered in a given datacenter. By default, the
  datacenter of the local agent is queried.

  To retrieve the list of nodes:

      $ consul catalog nodes

  To print detailed information including full node IDs, tagged addresses, and
  metadata information:

      $ consul catalog nodes -detailed

  To list nodes which are running a particular service:

      $ consul catalog nodes -service=web

  To filter by node metadata:

      $ consul catalog nodes -node-meta="foo=bar"

  To sort nodes by estimated round-trip time from node-web:

      $ consul catalog nodes -near=node-web

  For a full list of options and examples, please see the Consul documentation.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *CatalogListNodesCommand) Run(args []string) int {
	f := c.BaseCommand.NewFlagSet(c)

	detailed := f.Bool("detailed", false, "Output detailed information about "+
		"the nodes including their addresses and metadata.")

	near := f.String("near", "", "Node name to sort the node list in ascending "+
		"order based on estimated round-trip time from that node. "+
		"Passing \"_agent\" will use this agent's node for sorting.")

	nodeMeta := make(map[string]string)
	f.Var((*configutil.FlagMapValue)(&nodeMeta), "node-meta", "Metadata to "+
		"filter nodes with the given `key=value` pairs. This flag may be "+
		"specified multiple times to filter on multiple sources of metadata.")

	service := f.String("service", "", "Service `id or name` to filter nodes. "+
		"Only nodes which are providing the given service will be returned.")

	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}

	if l := len(f.Args()); l > 0 {
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 0, got %d)", l))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var nodes []*api.Node
	if *service != "" {
		services, _, err := client.Catalog().Service(*service, "", &api.QueryOptions{
			Near:     *near,
			NodeMeta: nodeMeta,
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error listing nodes for service: %s", err))
			return 1
		}

		nodes = make([]*api.Node, len(services))
		for i, s := range services {
			nodes[i] = &api.Node{
				ID:              s.ID,
				Node:            s.Node,
				Address:         s.Address,
				Datacenter:      s.Datacenter,
				TaggedAddresses: s.TaggedAddresses,
				Meta:            s.NodeMeta,
				CreateIndex:     s.CreateIndex,
				ModifyIndex:     s.ModifyIndex,
			}
		}
	} else {
		nodes, _, err = client.Catalog().Nodes(&api.QueryOptions{
			Near:     *near,
			NodeMeta: nodeMeta,
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error listing nodes: %s", err))
			return 1
		}
	}

	// Handle the edge case where there are no nodes that match the query.
	if len(nodes) == 0 {
		c.UI.Error("No nodes match the given query - try expanding your search.")
		return 0
	}

	output, err := printNodes(nodes, *detailed)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error printing nodes: %s", err))
		return 1
	}

	c.UI.Info(output)

	return 0
}

func (c *CatalogListNodesCommand) Synopsis() string {
	return "Lists all nodes in the given datacenter"
}

// printNodes accepts a list of nodes and prints information in a tabular
// format about the nodes.
func printNodes(nodes []*api.Node, detailed bool) (string, error) {
	var result []string
	if detailed {
		result = detailedNodes(nodes)
	} else {
		result = simpleNodes(nodes)
	}

	return columnize.SimpleFormat(result), nil
}

func detailedNodes(nodes []*api.Node) []string {
	result := make([]string, 0, len(nodes)+1)
	header := "Node|ID|Address|DC|TaggedAddresses|Meta"
	result = append(result, header)

	for _, node := range nodes {
		result = append(result, fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			node.Node, node.ID, node.Address, node.Datacenter,
			mapToKV(node.TaggedAddresses, ", "), mapToKV(node.Meta, ", ")))
	}

	return result
}

func simpleNodes(nodes []*api.Node) []string {
	result := make([]string, 0, len(nodes)+1)
	header := "Node|ID|Address|DC"
	result = append(result, header)

	for _, node := range nodes {
		// Shorten the ID in non-detailed mode to just the first octet.
		id := node.ID
		idx := strings.Index(id, "-")
		if idx > 0 {
			id = id[0:idx]
		}
		result = append(result, fmt.Sprintf("%s|%s|%s|%s",
			node.Node, id, node.Address, node.Datacenter))
	}

	return result
}
