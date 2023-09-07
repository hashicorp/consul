// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"

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

	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

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

	res, _, err := peerings.List(context.Background(), &api.QueryOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing peerings: %s", err))
		return 1
	}

	list := peeringList(res)
	sort.Sort(list)

	if c.format == peering.PeeringFormatJSON {
		output, err := json.Marshal(list)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error marshalling JSON: %s", err))
			return 1
		}
		c.UI.Output(string(output))
		return 0
	}

	if len(res) == 0 {
		c.UI.Info(fmt.Sprintf("There are no peering connections."))
		return 0
	}

	result := make([]string, 0, len(list))
	// TODO(peering): consider adding more StreamStatus fields here
	header := "Name\x1fState\x1fImported Svcs\x1fExported Svcs\x1fMeta"
	result = append(result, header)
	for _, peer := range list {
		metaPairs := make([]string, 0, len(peer.Meta))
		for k, v := range peer.Meta {
			metaPairs = append(metaPairs, fmt.Sprintf("%s=%s", k, v))
		}
		meta := strings.Join(metaPairs, ",")
		line := fmt.Sprintf("%s\x1f%s\x1f%d\x1f%d\x1f%s",
			peer.Name, peer.State, len(peer.StreamStatus.ImportedServices), len(peer.StreamStatus.ExportedServices), meta)
		result = append(result, line)
	}

	output := columnize.Format(result, &columnize.Config{Delim: string([]byte{0x1f})})
	c.UI.Output(output)

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "List peering connections"
	help     = `
Usage: consul peering list [options]

  List all peering connections.  The results will be filtered according
  to ACL policy configuration.

  Example:

    $ consul peering list
`
)

// peeringList applies sort.Interface to a list of peering connections for sorting by name.
type peeringList []*api.Peering

func (d peeringList) Len() int           { return len(d) }
func (d peeringList) Less(i, j int) bool { return d[i].Name < d[j].Name }
func (d peeringList) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
