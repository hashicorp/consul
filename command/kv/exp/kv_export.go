// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package exp

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/kv/impexp"
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

	key := ""
	// Check for arg validation
	args = c.flags.Args()
	switch len(args) {
	case 0:
		key = ""
	case 1:
		key = args[0]
	default:
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 1, got %d)", len(args)))
		return 1
	}

	// This is just a "nice" thing to do. Since pairs cannot start with a /, but
	// users will likely put "/" or "/foo", lets go ahead and strip that for them
	// here.
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	pairs, _, err := client.KV().List(key, &api.QueryOptions{
		AllowStale: c.http.Stale(),
	})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	exported := make([]*impexp.Entry, len(pairs))
	for i, pair := range pairs {
		exported[i] = impexp.ToEntry(pair)
	}

	marshaled, err := json.MarshalIndent(exported, "", "\t")
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error exporting KV data: %s", err))
		return 1
	}

	c.UI.Info(string(marshaled))

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Exports a tree from the KV store as JSON"
	help     = `
Usage: consul kv export [KEY_OR_PREFIX]

  Retrieves key-value pairs for the given prefix from Consul's key-value store,
  and writes a JSON representation to stdout. This can be used with the command
  "consul kv import" to move entire trees between Consul clusters.

      $ consul kv export vault

  For a full list of options and examples, please see the Consul documentation.
`
)
