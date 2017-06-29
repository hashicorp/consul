package command

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/configutil"
)

// CatalogNodesCommand is a Command implementation that is used to fetch all the
// datacenters the agent knows about.
type CatalogNodesCommand struct {
	BaseCommand
}

func (c *CatalogNodesCommand) Help() string {
	helpText := `
Usage: consul catalog datacenters [options]

  Retrieves the list nodes registered in a given datacenter. By default, the
  datacenter of the local agent is queried.

  To retrieve the list of nodes:

      $ consul catalog nodes

  For a full list of options and examples, please see the Consul documentation.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *CatalogNodesCommand) Run(args []string) int {
	f := c.BaseCommand.NewFlagSet(c)

	detailed := f.Bool("detailed", false, "Output detailed information about "+
		"the nodes including their addresses and metadata.")

	near := f.String("near", "", "Node name to sort the node list in ascending "+
		"order based on estimated round-trip time from that node. "+
		"Passing \"_agent\" will use this agent's node for sorting.")

	nodeMeta := make(map[string]string)
	f.Var((*configutil.FlagMapValue)(&nodeMeta), "node-meta", "Metadata to filter nodes with the "+
		"given `key=value` pairs.")

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

	nodes, _, err := client.Catalog().Nodes(&api.QueryOptions{
		Near:     *near,
		NodeMeta: nodeMeta,
	})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing nodes: %s", err))
	}

	if *detailed {
		var b bytes.Buffer
		for i, node := range nodes {
			if err := prettyNode(&b, node); err != nil {
				c.UI.Error(fmt.Sprintf("Error rendering node: %s", err))
				return 1
			}

			c.UI.Info(b.String())
			b.Reset()

			if i < len(nodes)-1 {
				c.UI.Info("")
			}
		}
	} else {
		for _, node := range nodes {
			name := node.Node
			if name == "" {
				name = node.ID
			}
			c.UI.Info(name)
		}
	}

	return 0
}

func (c *CatalogNodesCommand) Synopsis() string {
	return "Lists all nodes in the given datacenter"
}

// mapToKV converts a map[string]string into a human-friendly key=value list,
// sorted by name.
func mapToKV(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := make([]string, len(keys))
	for i, k := range keys {
		r[i] = fmt.Sprintf("%s=%s", k, m[k])
	}
	return strings.Join(r, ", ")
}

// prettyNode prints a node for human consumption.
func prettyNode(w io.Writer, node *api.Node) error {
	tw := tabwriter.NewWriter(w, 0, 2, 6, ' ', 0)
	fmt.Fprintf(tw, "ID\t%s\n", node.ID)
	if node.Node != "" {
		fmt.Fprintf(tw, "Node\t%s\n", node.Node)
	}
	fmt.Fprintf(tw, "Address\t%s\n", node.Address)
	fmt.Fprintf(tw, "Datacenter\t%s\n", node.Datacenter)
	if len(node.TaggedAddresses) > 0 {
		fmt.Fprintf(tw, "TaggedAddresses\t%s\n", mapToKV(node.TaggedAddresses))
	}
	if len(node.Meta) > 0 {
		fmt.Fprintf(tw, "Meta\t%s\n", mapToKV(node.Meta))
	}
	fmt.Fprintf(tw, "CreateIndex\t%d\n", node.CreateIndex)
	fmt.Fprintf(tw, "ModifyIndex\t%d", node.ModifyIndex)
	return tw.Flush()
}
