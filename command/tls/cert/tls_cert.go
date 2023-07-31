package cert

import (
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
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

const synopsis = `Helpers for certificates`
const help = `
Usage: consul tls cert <subcommand> [options]

  This command has subcommands for interacting with certificates

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Create a certificate

    $ consul tls cert create -server
    ==> saved dc1-server-consul.pem
    ==> saved dc1-server-consul-key.pem

  Create a certificate with your own CA:

    $ consul tls cert create -server -ca my-ca.pem -key my-ca-key.pem
    ==> saved dc1-server-consul.pem
    ==> saved dc1-server-consul-key.pem

  For more examples, ask for subcommand help or view the documentation.
`
