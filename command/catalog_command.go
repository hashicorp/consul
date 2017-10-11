package command

import (
	"github.com/mitchellh/cli"
)

var _ cli.Command = (*CatalogCommand)(nil)

type CatalogCommand struct {
	BaseCommand
}

func (c *CatalogCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *CatalogCommand) Help() string {
	c.InitFlagSet()
	return c.HelpCommand(`
Usage: consul catalog <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's catalog. The
  catalog should not be confused with the agent, although the APIs and
  responses may be similar.

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  List all datacenters:

      $ consul catalog datacenters

  List all nodes:

      $ consul catalog nodes

  List all services:

      $ consul catalog services

  For more examples, ask for subcommand help or view the documentation.

`)
}

func (c *CatalogCommand) Synopsis() string {
	return "Interact with the catalog"
}
