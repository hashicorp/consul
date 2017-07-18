package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

// KVCommand is a Command implementation that just shows help for
// the subcommands nested below it.
type KVCommand struct {
	BaseCommand
}

func (c *KVCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *KVCommand) Help() string {
	helpText := `
Usage: consul kv <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's key-value
  store. Here are some simple examples, and more detailed examples are
  available in the subcommands or the documentation.

  Create or update the key named "redis/config/connections" with the value "5":

      $ consul kv put redis/config/connections 5

  Read this value back:

      $ consul kv get redis/config/connections

  Or get detailed key information:

      $ consul kv get -detailed redis/config/connections

  Finally, delete the key:

      $ consul kv delete redis/config/connections

  For more examples, ask for subcommand help or view the documentation.

`
	return strings.TrimSpace(helpText)
}

func (c *KVCommand) Synopsis() string {
	return "Interact with the key-value store"
}
