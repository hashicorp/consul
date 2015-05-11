package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
)

// ConfigTestCommand is a Command implementation that is used to
// verify config files
type ConfigTestCommand struct {
	Ui cli.Ui
}

func (c *ConfigTestCommand) Help() string {
	helpText := `
Usage: consul configtest [options]

  Performs a basic sanity test on Consul configuration files. For each file
  or directory given, the configtest command will attempt to parse the
  contents just as the "consul agent" command would, and catch any errors.
  This is useful to do a test of the configuration only, without actually
  starting the agent.

  Returns 0 if the configuration is valid, or 1 if there are problems.

Options:

  -config-file=foo         Path to a JSON file to read configuration from.
                           This can be specified multiple times.
  -config-dir=foo          Path to a directory to read configuration files
                           from. This will read every file ending in ".json"
                           as configuration in this directory in alphabetical
                           order.
	`
	return strings.TrimSpace(helpText)
}

func (c *ConfigTestCommand) Run(args []string) int {
	var configFiles []string
	cmdFlags := flag.NewFlagSet("configtest", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.Var((*agent.AppendSliceValue)(&configFiles), "config-file", "json file to read config from")
	cmdFlags.Var((*agent.AppendSliceValue)(&configFiles), "config-dir", "directory of json files to read")
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	if len(configFiles) <= 0 {
		c.Ui.Error("Must specify config using -config-file or -config-dir")
		return 1
	}

	_, err := agent.ReadConfigPaths(configFiles)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Config validation failed: %v", err.Error()))
		return 1
	}
	return 0
}

func (c *ConfigTestCommand) Synopsis() string {
	return "Validate config file"
}
