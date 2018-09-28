package register

import (
	"flag"

	//"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
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

	// flags
	flagMeta map[string]string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.Var((*flags.FlagMapValue)(&c.flagMeta), "meta",
		"Metadata to set on the intention, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple meta fields.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Check for arg validation
	args = c.flags.Args()
	if len(args) == 0 {
		c.UI.Error("Service registration requires at least one argument.")
		return 1
	}

	/*
		ixns, err := c.ixnsFromArgs(args)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error: %s", err))
			return 1
		}

		// Create and test the HTTP client
		/*
			client, err := c.http.APIClient()
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
				return 1
			}
	*/

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Create intentions for service connections."
const help = `
Usage: consul intention create [options] SRC DST
Usage: consul intention create [options] -file FILE...

  Create one or more intentions. The data can be specified as a single
  source and destination pair or via a set of files when the "-file" flag
  is specified.

      $ consul intention create web db

  To consume data from a set of files:

      $ consul intention create -file one.json two.json

  When specifying the "-file" flag, "-" may be used once to read from stdin:

      $ echo "{ ... }" | consul intention create -file -

  An "allow" intention is created by default (whitelist). To create a
  "deny" intention, the "-deny" flag should be specified.

  If a conflicting intention is found, creation will fail. To replace any
  conflicting intentions, specify the "-replace" flag. This will replace any
  conflicting intentions with the intention specified in this command.
  Metadata and any other fields of the previous intention will not be
  preserved.

  Additional flags and more advanced use cases are detailed below.
`
