// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package list

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"

	"github.com/hashicorp/consul/command/flags"
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
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	ixns, _, err := client.Connect().Intentions(nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to retrieve the intentions list: %s", err))
		return 1
	}

	if len(ixns) == 0 {
		c.UI.Error(fmt.Sprintf("There are no intentions."))
		return 2
	}

	result := make([]string, 0, len(ixns))
	header := "ID\x1fSource\x1fAction\x1fDestination\x1fPrecedence"
	result = append(result, header)
	for _, ixn := range ixns {
		line := fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%d",
			ixn.ID, ixn.SourceName, ixn.Action, ixn.DestinationName, ixn.Precedence)
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
	return c.help
}

const (
	synopsis = "List intentions."
	help     = `
Usage: consul intention list

  List all intentions.
`
)
