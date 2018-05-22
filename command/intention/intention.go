package intention

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

const synopsis = "Interact with Connect service intentions"
const help = `
Usage: consul intention <subcommand> [options] [args]

  This command has subcommands for interacting with intentions. Intentions
  are permissions describing which services are allowed to communicate via
  Connect. Here are some simple examples, and more detailed examples are
  available in the subcommands or the documentation.

  Create an intention to allow "web" to talk to "db":

      $ consul intention create web db

  Test whether a "web" is allowed to connect to "db":

      $ consul intention check web db

  Find all intentions for communicating to the "db" service:

      $ consul intention match db

  For more examples, ask for subcommand help or view the documentation.
`
