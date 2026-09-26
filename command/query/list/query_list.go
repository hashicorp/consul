// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/catalog"
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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	queries, _, err := client.PreparedQuery().List(&api.QueryOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing prepared queries: %v", err))
		return 1
	}

	if len(queries) > 0 {
		output, err := printQueries(queries)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error formatting prepared queries: %s", err))
			return 1
		}

		c.UI.Info(output)
	}

	return 0
}

func printQueries(queries []*api.PreparedQueryDefinition) (string, error) {
	var result = detailedQueries(queries)

	return columnize.Format(result, &columnize.Config{Delim: string([]byte{0x1f})}), nil
}

func detailedQueries(queries []*api.PreparedQueryDefinition) []string {
	result := make([]string, 0, len(queries)+1)
	header := catalog.QueriesHeader()
	result = append(result, header)

	for _, query := range queries {
		result = append(result, catalog.QueryRow(query))
	}

	return result
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "List prepared queries"
	help     = `
Usage: consul query list [options]

  Lists all of the prepared queries.

  Example:

    $ consul query list

`
)
