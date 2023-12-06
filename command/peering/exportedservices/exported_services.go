// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/peering"
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

	name   string
	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.name, "name", "", "(Required) The local name assigned to the peer cluster.")

	c.flags.StringVar(
		&c.format,
		"format",
		peering.PeeringFormatPretty,
		fmt.Sprintf("Output format {%s} (default: %s)", strings.Join(peering.GetSupportedFormats(), "|"), peering.PeeringFormatPretty),
	)

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.name == "" {
		c.UI.Error("Missing the required -name flag")
		return 1
	}

	if !peering.FormatIsValid(c.format) {
		c.UI.Error(fmt.Sprintf("Invalid format, valid formats are {%s}", strings.Join(peering.GetSupportedFormats(), "|")))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	peerings := client.Peerings()

	res, _, err := peerings.Read(context.Background(), c.name, &api.QueryOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading peering: %s", err))
		return 1
	}

	if res == nil {
		c.UI.Error(fmt.Sprintf("No peering with name %s found.", c.name))
		return 1
	}

	// Convert service to serviceID
	services := make([]structs.ServiceID, 0, len(res.StreamStatus.ExportedServices))
	for _, svc := range res.StreamStatus.ExportedServices {
		services = append(services, structs.ServiceIDFromString(svc))
	}

	if c.format == peering.PeeringFormatJSON {
		output, err := json.Marshal(services)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error marshalling JSON: %s", err))
			return 1
		}
		c.UI.Output(string(output))
		return 0
	}

	c.UI.Output(formatExportedServices(services))

	return 0
}

func formatExportedServices(services []structs.ServiceID) string {
	if len(services) == 0 {
		return ""
	}

	result := make([]string, 0, len(services)+1)

	if services[0].EnterpriseMeta.ToEnterprisePolicyMeta() != nil {
		result = append(result, "Partition\x1fNamespace\x1fService Name")
	}

	for _, svc := range services {
		if svc.EnterpriseMeta.ToEnterprisePolicyMeta() == nil {
			result = append(result, svc.ID)
		} else {
			result = append(result, fmt.Sprintf("%s\x1f%s\x1f%s", svc.EnterpriseMeta.PartitionOrDefault(), svc.EnterpriseMeta.NamespaceOrDefault(), svc.ID))
		}
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
	synopsis = "Lists exported services to a peer"
	help     = `
Usage: consul peering exported-services [options] -name <peer name>

  Lists services exported to the peer with the provided name. If the peer is not found,
  the command exits with a non-zero code. The result is filtered according
  to ACL policy configuration.

  Example:

    $ consul peering exported-services -name west-dc
`
)
