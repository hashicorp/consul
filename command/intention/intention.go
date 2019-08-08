package intention

import (
	"fmt"
	"github.com/hashicorp/consul/api"
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

// SourceTypeUsageAbbrev is the usage of the -source-type flag in all commands except
// for intention create which has a more detailed usage.
const SourceTypeUsageAbbrev = "Type of SRC. One of consul (default)," +
	" external-trust-domain or external-uri."

// SourceTypeFlagName is the name of the source type flag.
const SourceTypeFlagName = "source-type"

// ValidateSourceTypeFlag returns an error if srcType is not a valid type.
// If valid it returns the corresponding enum value.
func ValidateSourceTypeFlag(srcType string) (api.IntentionSourceType, error) {
	ist := api.IntentionSourceType(srcType)
	switch ist {
	case api.IntentionSourceConsul,
		api.IntentionSourceExternalTrustDomain,
		api.IntentionSourceExternalURI:
		return ist, nil
	default:
		return ist, fmt.Errorf("-%s %q is not supported: must be set to %s, %s or %s",
			SourceTypeFlagName,
			srcType,
			api.IntentionSourceConsul,
			api.IntentionSourceExternalTrustDomain,
			api.IntentionSourceExternalURI)
	}
}
