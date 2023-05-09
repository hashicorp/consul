package ca

import (
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New() *cmd {
	return &cmd{}
}

type cmd struct{}

func (c *cmd) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(help, nil)
}

const synopsis = `Helpers for CAs`
const help = `
Usage: consul tls ca <subcommand> [options] filename-prefix

  This command has subcommands for interacting with Certificate Authorities.

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Create a CA

    $ consul tls ca create
    ==> saved consul-agent-ca.pem
    ==> saved consul-agent-ca-key.pem

  For more examples, ask for subcommand help or view the documentation.
`
