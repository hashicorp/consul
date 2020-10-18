package services

import (
	"flag"
	"fmt"
	"sort"
	"strings"

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
	node     string
	nodeMeta map[string]string
	tags     bool
	service  string
}

type serviceInfo struct {
	address string
	port    int
	tags    []string
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
	flags.Merge(c.flags, c.http.NamespaceFlags())
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
					address: s.Address,
					port:    s.ServicePort,
					tags:    s.ServiceTags,
				}
			}
		}
	} else {
		catalogServices, _, err := client.Catalog().Services(&api.QueryOptions{
			NodeMeta: c.nodeMeta,
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error listing services: %s", err))
			return 1
		}
		if catalogServices != nil {
			services = make(map[string]*serviceInfo, len(catalogServices))
			for s, tags := range catalogServices {
				services[s] = &serviceInfo{
					tags: tags,
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

	if c.service != "" {
		output := formatService(c.service, services[c.service], c.tags)
		c.UI.Info(output)
	} else if c.tags {
		output := formatServicesWithTags(order, services)
		c.UI.Info(output)
	} else {
		for _, s := range order {
			c.UI.Info(s)
		}
	}

	return 0
}

func formatService(service string, s *serviceInfo, tags bool) string {
	result := make([]string, 0, 2)
	var header string
	if tags {
		header = "Service\x1fAddress\x1fPort\x1fTags"
		result = append(result, header)
		sort.Strings(s.tags)
		result = append(result, fmt.Sprintf("%s\x1f%s\x1f%d\x1f%s", service, s.address, s.port, strings.Join(s.tags, ",")))
	} else {
		header = "Service\x1fAddress\x1fPort"
		result = append(result, header)
		result = append(result, fmt.Sprintf("%s\x1f%s\x1f%d", service, s.address, s.port))
	}
	return formatColumns(result)
}

func formatServicesWithTags(sortedServices []string, services map[string]*serviceInfo) string {
	result := make([]string, 0, len(services)+1)
	header := "Service\x1fTags"
	result = append(result, header)
	for _, s := range sortedServices {
		sort.Strings(services[s].tags)
		result = append(result, fmt.Sprintf("%s\x1f%s", s, strings.Join(services[s].tags, ",")))
	}
	return formatColumns(result)
}

func formatColumns(s []string) string {
	return columnize.Format(s, &columnize.Config{Delim: string([]byte{0x1f})})
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
