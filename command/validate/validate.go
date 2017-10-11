package validate

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.initFlags()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	quiet bool
}

func (c *cmd) initFlags() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.quiet, "quiet", false,
		"When given, a successful run will produce no output.")
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	configFiles := c.flags.Args()
	if len(configFiles) < 1 {
		c.UI.Error("Must specify at least one config file or directory")
		return 1
	}
	b, err := config.NewBuilder(config.Flags{ConfigFiles: configFiles})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Config validation failed: %v", err.Error()))
		return 1
	}
	if _, err := b.BuildAndValidate(); err != nil {
		c.UI.Error(fmt.Sprintf("Config validation failed: %v", err.Error()))
		return 1
	}
	if !c.quiet {
		c.UI.Output("Configuration is valid!")
	}
	return 0
}

func (c *cmd) Synopsis() string {
	return "Validate config files/directories"
}

func (c *cmd) Help() string {
	s := `Usage: consul validate [options] FILE_OR_DIRECTORY...

  Performs a basic sanity test on Consul configuration files. For each file
  or directory given, the validate command will attempt to parse the
  contents just as the "consul agent" command would, and catch any errors.
  This is useful to do a test of the configuration only, without actually
  starting the agent.

  Returns 0 if the configuration is valid, or 1 if there are problems.`

	return flags.Usage(s, c.flags, nil, nil)
}
