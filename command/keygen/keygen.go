// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package keygen

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	key := make([]byte, 32)
	n, err := rand.Reader.Read(key)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading random data: %s", err))
		return 1
	}
	if n != 32 {
		c.UI.Error(fmt.Sprintf("Couldn't read enough entropy. Generate more entropy!"))
		return 1
	}

	c.UI.Output(base64.StdEncoding.EncodeToString(key))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Generates a new encryption key"
const help = `
Usage: consul keygen

  Generates a new 32-byte encryption key that can be used to configure the
  agent to encrypt traffic. The output of this command is already
  in the proper format that the agent expects.
`
