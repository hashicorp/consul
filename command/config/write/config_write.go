// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package write

import (
	"flag"
	"fmt"
	"io"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/config"
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

	cas         bool
	modifyIndex uint64
	testStdin   io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	c.flags.BoolVar(&c.cas, "cas", false,
		"Perform a Check-And-Set operation. Specifying this value also "+
			"requires the -modify-index flag to be set. The default value "+
			"is false.")
	c.flags.Uint64Var(&c.modifyIndex, "modify-index", 0,
		"Unsigned integer representing the ModifyIndex of the config entry. "+
			"This is used in combination with the -cas flag.")
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
		c.UI.Error("Must provide exactly one positional argument to specify the config entry to write")
		return 1
	}

	data, err := helpers.LoadDataSourceNoRaw(args[0], c.testStdin)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to load data: %v", err))
		return 1
	}

	entry, err := helpers.ParseConfigEntry(data)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to decode config entry input: %v", err))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	entries := client.ConfigEntries()

	written := false
	if c.cas {
		written, _, err = entries.CAS(entry, c.modifyIndex, nil)
	} else {
		written, _, err = entries.Set(entry, nil)
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing config entry %s/%s: %v", entry.GetKind(), entry.GetName(), err))
		return 1
	}

	if !written {
		c.UI.Error(fmt.Sprintf("Config entry not updated: %s/%s", entry.GetKind(), entry.GetName()))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Config entry written: %s/%s", entry.GetKind(), entry.GetName()))

	if msg := config.KindSpecificWriteWarning(entry); msg != "" {
		c.UI.Warn("WARNING: " + msg)
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
	synopsis = "Create or update a centralized config entry"
	help     = `
Usage: consul config write [options] <configuration>

  Request a config entry to be created or updated. The configuration
  argument is either a file path or '-' to indicate that the config
  should be read from stdin. The data should be either in HCL or
  JSON form.

  Example (from file):

    $ consul config write web.service.hcl

  Example (from stdin):

    $ consul config write -
`
)
