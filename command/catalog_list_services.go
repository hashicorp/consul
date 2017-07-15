package command

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/configutil"
	"github.com/mitchellh/cli"
)

var _ cli.Command = (*CatalogListServicesCommand)(nil)

// CatalogListServicesCommand is a Command implementation that is used to fetch all the
// datacenters the agent knows about.
type CatalogListServicesCommand struct {
	BaseCommand
}

func (c *CatalogListServicesCommand) Help() string {
	helpText := `
Usage: consul catalog services [options]

  Retrieves the list services registered in a given datacenter. By default, the
  datacenter of the local agent is queried.

  To retrieve the list of services:

      $ consul catalog services

  To include the services' tags in the output:

      $ consul catalog services -tags

  To list services which run on a particular node:

      $ consul catalog services -node=web

  To filter services on node metadata:

      $ consul catalog services -node-meta="foo=bar"

  For a full list of options and examples, please see the Consul documentation.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *CatalogListServicesCommand) Run(args []string) int {
	f := c.BaseCommand.NewFlagSet(c)

	node := f.String("node", "", "Node `id or name` for which to list services.")

	nodeMeta := make(map[string]string)
	f.Var((*configutil.FlagMapValue)(&nodeMeta), "node-meta", "Metadata to "+
		"filter nodes with the given `key=value` pairs. If specified, only "+
		"services running on nodes matching the given metadata will be returned. "+
		"This flag may be specified multiple times to filter on multiple sources "+
		"of metadata.")

	tags := f.Bool("tags", false, "Display each service's tags as a "+
		"comma-separated list beside each service entry.")

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

	var services map[string][]string
	if *node != "" {
		catalogNode, _, err := client.Catalog().Node(*node, &api.QueryOptions{
			NodeMeta: nodeMeta,
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error listing services for node: %s", err))
			return 1
		}
		if catalogNode != nil {
			services = make(map[string][]string, len(catalogNode.Services))
			for _, s := range catalogNode.Services {
				services[s.Service] = append(services[s.Service], s.Tags...)
			}
		}
	} else {
		services, _, err = client.Catalog().Services(&api.QueryOptions{
			NodeMeta: nodeMeta,
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error listing services: %s", err))
			return 1
		}
	}

	// Handle the edge case where there are no services that match the query.
	if len(services) == 0 {
		c.UI.Error("No services match the given query - try expanding your search.")
		return 0
	}

	// Order the map for consistent output
	order := make([]string, 0, len(services))
	for k, _ := range services {
		order = append(order, k)
	}
	sort.Strings(order)

	if *tags {
		var b bytes.Buffer
		tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)
		for _, s := range order {
			sort.Strings(services[s])
			fmt.Fprintf(tw, "%s\t%s\n", s, strings.Join(services[s], ","))
		}
		if err := tw.Flush(); err != nil {
			c.UI.Error(fmt.Sprintf("Error flushing tabwriter: %s", err))
			return 1
		}
		c.UI.Output(strings.TrimSpace(b.String()))
	} else {
		for _, s := range order {
			c.UI.Output(s)
		}
	}

	return 0
}

func (c *CatalogListServicesCommand) Synopsis() string {
	return "Lists all registered services in a datacenter"
}
