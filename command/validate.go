package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/configutil"
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
	var configFiles []string
	var quiet bool

	f := c.BaseCommand.NewFlagSet(c)
	f.Var((*configutil.AppendSliceValue)(&configFiles), "config-file",
		"Path to a JSON file to read configuration from. This can be specified multiple times.")
	f.Var((*configutil.AppendSliceValue)(&configFiles), "config-dir",
		"Path to a directory to read configuration files from. This will read every file ending in "+
			".json as configuration in this directory in alphabetical order.")
	f.BoolVar(&quiet, "quiet", false,
		"When given, a successful run will produce no output.")
	c.BaseCommand.HideFlags("config-file", "config-dir")

	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}

	if len(f.Args()) > 0 {
		configFiles = append(configFiles, f.Args()...)
	}

	if len(configFiles) < 1 {
		c.UI.Error("Must specify at least one config file or directory")
		return 1
	}

	_, err := agent.ReadConfigPaths(configFiles)
	if err != nil {
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
