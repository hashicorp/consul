// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package importedservices

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"

	"github.com/hashicorp/go-bexpr"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
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

	importedServices, _, err := client.ImportedServices(nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading imported services: %v", err))
		return 1
	}

	var filterType []api.ImportedService
	filter, err := bexpr.CreateFilter(c.filter, nil, filterType)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error while creating filter: %s", err))
		return 1
	}

	raw, err := filter.Execute(importedServices)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error while filtering response: %s", err))
		return 1
	}

	filteredServices := raw.([]api.ImportedService)

	if len(filteredServices) == 0 {
		c.UI.Info("No imported services found")
		return 0
	}

	switch c.format == JSONFormat {
	case true:
		output, err := json.MarshalIndent(filteredServices, "", "    ")
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error marshalling JSON: %s", err))
			return 1
		}
		c.UI.Output(string(output))
		return 0

	default:
		c.UI.Output(formatImportedServices(filteredServices))
	}

	return 0
}

func formatImportedServices(services []api.ImportedService) string {
	result := make([]string, 0, len(services)+1)

	// Determine if we're in enterprise mode based on presence of partition/namespace
	hasEnterpriseMeta := len(services) > 0 && (services[0].Partition != "" || services[0].Namespace != "")

	// Build header - always show both peer and partition in enterprise mode
	if hasEnterpriseMeta {
		result = append(result, "Service\x1fSource Partition\x1fSource Peer\x1fNamespace")
	} else {
		result = append(result, "Service\x1fSource Peer")
	}

	for _, impService := range services {
		row := ""
		if hasEnterpriseMeta {
			row = fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s",
				impService.Service,
				impService.SourcePartition,
				impService.SourcePeer,
				impService.Namespace)
		} else {
			row = fmt.Sprintf("%s\x1f%s", impService.Service, impService.SourcePeer)
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
	synopsis = "Lists imported services"
	help     = `
Usage: consul services imported-services [options]

  Lists all the imported services and their sources. Wildcards and sameness groups(Enterprise) are expanded.

  Example:

    $ consul services imported-services
`
)
