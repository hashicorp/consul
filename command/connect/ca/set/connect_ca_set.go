package set

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/hashicorp/consul/api"
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
	configFile flags.StringValue
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.Var(&c.configFile, "config-file",
		"The path to the config file to use.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if c.configFile.String() == "" {
		c.UI.Error("The -config-file flag is required")
		return 1
	}

	bytes, err := ioutil.ReadFile(c.configFile.String())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading config file: %s", err))
		return 1
	}

	var config api.CAConfig
	if err := json.Unmarshal(bytes, &config); err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing config file: %s", err))
		return 1
	}

	// Set the new configuration.
	if _, err := client.Connect().CASetConfig(&config, nil); err != nil {
		c.UI.Error(fmt.Sprintf("Error setting CA configuration: %s", err))
		return 1
	}
	c.UI.Output("Configuration updated!")
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Modify the current Connect CA configuration"
const help = `
Usage: consul connect ca set-config [options]

  Modifies the current Connect Certificate Authority (CA) configuration.
`
