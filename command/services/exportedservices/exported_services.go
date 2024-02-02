// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/go-bexpr"
	"github.com/ryanuber/columnize"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

func getSupportedFormats() []string {
	return []string{PrettyFormat, JSONFormat}
}

func formatIsValid(f string) bool {
	for _, format := range getSupportedFormats() {
		if f == format {
			return true
		}
	}
	return false

}

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

	format string
	filter string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(
		&c.format,
		"format",
		PrettyFormat,
		fmt.Sprintf("Output format {%s} (default: %s)", strings.Join(getSupportedFormats(), "|"), PrettyFormat),
	)

	c.flags.StringVar(&c.filter, "filter", "", "go-bexpr filter string to filter the response")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if !formatIsValid(c.format) {
		c.UI.Error(fmt.Sprintf("Invalid format, valid formats are {%s}", strings.Join(getSupportedFormats(), "|")))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	exportedServices, _, err := client.ExportedServices(nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading exported services: %v", err))
		return 1
	}

	var filterType []api.ResolvedExportedService
	filter, err := bexpr.CreateFilter(c.filter, nil, filterType)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error while creating filter: %s", err))
		return 1
	}

	raw, err := filter.Execute(exportedServices)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error while filtering response: %s", err))
		return 1
	}

	filteredServices := raw.([]api.ResolvedExportedService)

	if len(filteredServices) == 0 {
		c.UI.Info("No exported services found")
		return 0
	}

	if c.format == JSONFormat {
		output, err := json.MarshalIndent(filteredServices, "", "    ")
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error marshalling JSON: %s", err))
			return 1
		}
		c.UI.Output(string(output))
		return 0
	}

	c.UI.Output(formatExportedServices(filteredServices))

	return 0
}

func formatExportedServices(services []api.ResolvedExportedService) string {
	result := make([]string, 0, len(services)+1)

	if services[0].Partition != "" {
		result = append(result, "Service\x1fPartition\x1fNamespace\x1fConsumer Peers\x1fConsumer Partitions")
	} else {
		result = append(result, "Service\x1fConsumer Peers")
	}

	for _, expService := range services {
		row := ""
		peers := strings.Join(expService.Consumers.Peers, ", ")
		partitions := strings.Join(expService.Consumers.Partitions, ", ")
		if expService.Partition != "" {
			row = fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%s", expService.Service, expService.Partition, expService.Namespace, peers, partitions)
		} else {
			row = fmt.Sprintf("%s\x1f%s", expService.Service, peers)
		}

		result = append(result, row)

	}

	return columnize.Format(result, &columnize.Config{Delim: string([]byte{0x1f})})
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Lists exported services"
	help     = `
Usage: consul services exported-services [options]

  Lists all the exported services and their consumers. Wildcards and sameness groups(Enterprise) are expanded.

  Example:

    $ consul services exported-services
`
)
