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
	"github.com/hashicorp/consul/lib/stringslice"
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

	format            string
	consumerPeer      string
	consumerPartition string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(
		&c.format,
		"format",
		PrettyFormat,
		fmt.Sprintf("Output format {%s} (default: %s)", strings.Join(getSupportedFormats(), "|"), PrettyFormat),
	)

	c.flags.StringVar(&c.consumerPeer, "consumerPeer", "", "If provided, output is filtered to only services which have the consumer peer")
	c.flags.StringVar(&c.consumerPartition, "consumerPartition", "", "If provided, output is filtered to only services which have the consumer partition")

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

	filteredServices := c.FilterResponse(exportedServices)
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
		result = append(result, "Service\x1fPartition\x1fNamespace\x1fConsumer")
	} else {
		result = append(result, "Service\x1fConsumer")
	}

	for _, expService := range services {
		service := ""
		if expService.Partition != "" {
			service = fmt.Sprintf("%s\x1f%s\x1f%s", expService.Service, expService.Partition, expService.Namespace)
		} else {
			service = expService.Service
		}

		for _, peer := range expService.Consumers.Peers {
			result = append(result, fmt.Sprintf("%s\x1fPeer: %s", service, peer))
		}
		for _, partition := range expService.Consumers.Partitions {
			result = append(result, fmt.Sprintf("%s\x1fPartition: %s", service, partition))
		}

	}

	return columnize.Format(result, &columnize.Config{Delim: string([]byte{0x1f})})
}

func (c *cmd) FilterResponse(exportedServices []api.ResolvedExportedService) []api.ResolvedExportedService {
	if c.consumerPartition == "" && c.consumerPeer == "" {
		return exportedServices
	}

	var resp []api.ResolvedExportedService

	for _, svc := range exportedServices {
		cloneSvc := api.ResolvedExportedService{
			Service:   svc.Service,
			Partition: svc.Partition,
			Namespace: svc.Namespace,
		}

		includeService := false

		if stringslice.Contains(svc.Consumers.Partitions, c.consumerPartition) {
			cloneSvc.Consumers.Partitions = []string{c.consumerPartition}
			includeService = true
		}
		if stringslice.Contains(svc.Consumers.Peers, c.consumerPeer) {
			cloneSvc.Consumers.Peers = []string{c.consumerPeer}
			includeService = true
		}

		if includeService {
			resp = append(resp, cloneSvc)
		}
	}

	return resp
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
