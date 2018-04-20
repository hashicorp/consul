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
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	// configFormat forces all config files to be interpreted as this
	// format independent of their extension.
	configFormat string
	quiet        bool
	help         string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.configFormat, "config-format", "",
		"Config files are in this format irrespective of their extension. Must be 'hcl' or 'json'")
	c.flags.BoolVar(&c.quiet, "quiet", false,
		"When given, a successful run will produce no output.")
	c.help = flags.Usage(help, c.flags)
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

	if c.configFormat != "" && c.configFormat != "json" && c.configFormat != "hcl" {
		c.UI.Error("-config-format must be either 'hcl' or 'json")
		return 1
	}

	b, err := config.NewBuilder(config.Flags{ConfigFiles: configFiles, ConfigFormat: &c.configFormat})
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
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Validate config files/directories"
const help = `
Usage: consul validate [options] FILE_OR_DIRECTORY...

  Performs a thorough sanity test on Consul configuration files. For each file
  or directory given, the validate command will attempt to parse the contents
  just as the "consul agent" command would, and catch any errors.

  This is useful to do a test of the configuration only, without actually
  starting the agent. This performs all of the validation the agent would, so
  this should be given the complete set of configuration files that are going
  to be loaded by the agent. This command cannot operate on partial
  configuration fragments since those won't pass the full agent validation.

  Returns 0 if the configuration is valid, or 1 if there are problems.
`
