// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package set

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
	configFile               flags.StringValue
	forceWithoutCrossSigning bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.Var(&c.configFile, "config-file",
		"The path to the config file to use.")
	c.flags.BoolVar(&c.forceWithoutCrossSigning, "force-without-cross-signing", false,
		"Indicates that the CA reconfiguration should go ahead even if the current "+
			"CA is unable to cross sign certificates. This risks temporary connection "+
			"failures during the rollout as new leafs will be rejected by proxies that "+
			"have not yet observed the new root cert but is the only option if a CA that "+
			"doesn't support cross signing needs to be reconfigured or mirated away from.")

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

	bytes, err := os.ReadFile(c.configFile.String())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading config file: %s", err))
		return 1
	}

	var config api.CAConfig
	if err := json.Unmarshal(bytes, &config); err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing config file: %s", err))
		return 1
	}
	config.ForceWithoutCrossSigning = c.forceWithoutCrossSigning

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
