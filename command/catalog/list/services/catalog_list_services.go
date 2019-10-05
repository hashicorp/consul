package services

import (
	"bytes"
	"flag"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
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
	node     string
	nodeMeta map[string]string
	tags     bool
	service  string
}

type serviceInfo struct {
	address	string
	port 	int
	tags	[]string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.node, "node", "",
		"Node `id or name` for which to list services.")
	c.flags.Var((*flags.FlagMapValue)(&c.nodeMeta), "node-meta", "Metadata to "+
		"filter nodes with the given `key=value` pairs. If specified, only "+
		"services running on nodes matching the given metadata will be returned. "+
		"This flag may be specified multiple times to filter on multiple sources "+
		"of metadata.")
	c.flags.BoolVar(&c.tags, "tags", false, "Display each service's tags as a "+
		"comma-separated list beside each service entry.")
	c.flags.StringVar(&c.service, "service", "", "Service `name` for which to list host and port.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if l := len(c.flags.Args()); l > 0 {
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 0, got %d)", l))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var services map[string]*serviceInfo
	if c.node != "" {
		catalogNode, _, err := client.Catalog().Node(c.node, &api.QueryOptions{
			NodeMeta: c.nodeMeta,
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error listing services for node: %s", err))
			return 1
		}
		if catalogNode != nil {
			services = make(map[string]*serviceInfo, len(catalogNode.Services))
			for _, s := range catalogNode.Services {
				services[s.Service] = &serviceInfo{
					address: s.Address,
					port:    s.Port,
					tags:    s.Tags,
				}
			}
		}
	} else if c.service != "" {
		catalogServices, _, err := client.Catalog().Service(c.service, "", nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error print service: %s", err))
			return 1
		}

		if catalogServices != nil {
			services = make(map[string]*serviceInfo, len(catalogServices))
			for _, s := range catalogServices {
				services[s.ServiceName] = &serviceInfo{
					address:	s.Address,
					port:		s.ServicePort,
					tags:		s.ServiceTags,
				}
			}
		}
	} else {
		allServices, _, err := client.Catalog().Services(&api.QueryOptions{
			NodeMeta: c.nodeMeta,
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error listing services: %s", err))
			return 1
		}
		if allServices != nil {
			services = make(map[string]*serviceInfo, len(allServices))
			for s, tags := range allServices {
				services[s] = &serviceInfo{
					tags:	tags,
				}
			}
		}
	}

	// Handle the edge case where there are no services that match the query.
	if len(services) == 0 {
		c.UI.Error("No services match the given query - try expanding your search.")
		return 0
	}

	// Order the map for consistent output
	order := make([]string, 0, len(services))
	for k := range services {
		order = append(order, k)
	}
	sort.Strings(order)

	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)

	if c.service != "" {
		s := c.service
		if c.tags {
			sort.Strings(services[s].tags)
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", s, services[s].address, services[s].port, strings.Join(services[s].tags, ","))
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%d\n", s, services[s].address, services[s].port)
		}
		return c.flushAndPrint(tw, &b)
	} else if c.tags {
		for _, s := range order {
			sort.Strings(services[s].tags)
			fmt.Fprintf(tw, "%s\t%s\n", s, strings.Join(services[s].tags, ","))
		}
		return c.flushAndPrint(tw, &b)
	} else {
		for _, s := range order {
			c.UI.Output(s)
		}
	}

	return 0
}

func (c *cmd) flushAndPrint(tw *tabwriter.Writer, b *bytes.Buffer) int {
	if err := tw.Flush(); err != nil {
		c.UI.Error(fmt.Sprintf("Error flushing tabwriter: %s", err))
		return 1
	}
	c.UI.Output(strings.TrimSpace(b.String()))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Lists all registered services in a datacenter"
const help = `
Usage: consul catalog services [options]

  Retrieves the list services registered in a given datacenter. By default, the
  datacenter of the local agent is queried.

  To retrieve the list of services:

      $ consul catalog services

  To retrieve information about particular service:

      $ consul catalog services -service=web

  To include the services' tags in the output:

      $ consul catalog services -tags

  To list services which run on a particular node:

      $ consul catalog services -node=web

  To filter services on node metadata:

      $ consul catalog services -node-meta="foo=bar"

  For a full list of options and examples, please see the Consul documentation.
`
