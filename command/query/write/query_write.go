// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package write

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
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

	testStdin io.Reader
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

	args = c.flags.Args()
	if len(args) != 1 {
		c.UI.Error("Must provide exactly one positional argument to specify the prepared query to write")
		return 1
	}

	if len(args[0]) == 0 {
		c.UI.Error("Failed to load data: must specify a file path or '-' for stdin")
		return 1
	}

	data, err := helpers.LoadDataSourceNoRaw(args[0], c.testStdin)
	if err != nil {
		c.UI.Error("Failed to read input data")
		return 1
	}

	var inputQuery api.PreparedQueryDefinition
	err = json.Unmarshal([]byte(data), &inputQuery)
	if err != nil {
		c.UI.Error("Failed to unmarshal input data")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	if inputQuery.ID != "" {
		_, err = client.PreparedQuery().Update(&inputQuery, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error updating prepared query %s: %v", inputQuery.ID, err))
			return 1
		}
		c.UI.Info(fmt.Sprintf("Query Updated: %s", inputQuery.ID))
	} else {
		ID, _, err := client.PreparedQuery().Create(&inputQuery, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error creating prepared query %s: %v", ID, err))
			return 1
		}
		c.UI.Info(fmt.Sprintf("Query Created: %s", ID))
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Create or update a prepared query"
	help     = `
Usage: consul query write [options] <configuration>

  Request a prepared query to be created or updated. The configuration
  argument is either a file path or '-' to indicate that the config
  should be read from stdin. The data should be either in JSON form.

  Example (from file):

    $ consul query write query.json

  Example (from stdin):

    $ echo '{"Name":"example-query","Service":{"Service":"example-service","OnlyPassing":true},"DNS":{"TTL":"60s"}}'|consul query write -

  Example (from stdin) updating existing query:

    $ echo '{"ID": "ae8d19b5-6f25-c50f-7ff4-53c0b9ccdf78",Name":"example-query","Service":{"Service":"example-service","OnlyPassing":true},"DNS":{"TTL":"60s"}}'|consul query write -

`
)
