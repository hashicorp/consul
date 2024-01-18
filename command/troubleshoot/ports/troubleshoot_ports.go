// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ports

import (
	"flag"
	"fmt"
	"github.com/hashicorp/consul/troubleshoot/ports"
	"os"

	"github.com/hashicorp/consul/command/cli"
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
	help  string

	// flags
	host string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.host, "host", os.Getenv("CONSUL_HTTP_ADDR"), "The consul server host")

	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {

	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	if c.host == "" {
		c.UI.Error("-host is required.")
		return 1
	}
	ports.Troubleshoot(c.host)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Troubleshoots ports of consul server"
	help     = `
Usage: consul troubleshoot ports
`
)
