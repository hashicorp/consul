package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/config"
)

// ValidateCommand is a Command implementation that is used to
// verify config files
type ValidateCommand struct {
	BaseCommand
}

func (c *ValidateCommand) Help() string {
	helpText := `
Usage: consul validate [options] FILE_OR_DIRECTORY...

  Performs a basic sanity test on Consul configuration files. For each file
  or directory given, the validate command will attempt to parse the
  contents just as the "consul agent" command would, and catch any errors.
  This is useful to do a test of the configuration only, without actually
  starting the agent.

  Returns 0 if the configuration is valid, or 1 if there are problems.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *ValidateCommand) Run(args []string) int {
	var quiet bool

	f := c.BaseCommand.NewFlagSet(c)
	f.BoolVar(&quiet, "quiet", false,
		"When given, a successful run will produce no output.")

	if err := c.BaseCommand.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	configFiles := f.Args()
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

	if !quiet {
		c.UI.Output("Configuration is valid!")
	}
	return 0
}

func (c *ValidateCommand) Synopsis() string {
	return "Validate config files/directories"
}
