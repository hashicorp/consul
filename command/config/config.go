// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

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

const synopsis = "Interact with Consul's Centralized Configurations"
const help = `
Usage: consul config <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's Centralized
  Configuration system. Here are some simple examples, and more detailed
  examples are available in the subcommands or the documentation.

  Write a config:

    $ consul config write web.serviceconf.hcl

  Read a config:

    $ consul config read -kind service-defaults -name web

  List all configs for a type:

    $ consul config list -kind service-defaults

  Delete a config:

    $ consul config delete -kind service-defaults -name web

  For more examples, ask for subcommand help or view the documentation.
`

// KindSpecificWarning returns a warning message for the given config entry.
// Use this to inform the user of (un)recommended settings when they read or
// write config entries with the CLI.
func KindSpecificWarning(entry api.ConfigEntry) string {
	switch e := entry.(type) {
	case *api.ServiceConfigEntry:
		if e.MutualTLSMode == api.MutualTLSModePermissive {
			return "Found MutualTLSMode=permissive. This mode is insecure." +
				" We recommend transitioning this to MutualTLSMode=strict."
		}
	case *api.ProxyConfigEntry:
		if e.MutualTLSMode == api.MutualTLSModePermissive {
			return "Found MutualTLSMode=permissive. This mode is insecure." +
				" Setting this mode in proxy-defaults enables this insecure mode by default for all services." +
				" We recommend setting this to MutualTLSMode=strict."
		}
	}
	return ""
}

// KindSpecificWarnings returns warning messages for the given config entries.
// Use this to inform the user of (un)recommended settings when they list
// config entries with the CLI.
//
// When updating this, prefer to squash warnings down into fewer messages to
// avoid flooding the user with noisey warnings.
func KindSpecificWarnings(entries []api.ConfigEntry) []string {
	var result []string
	servicesInPermissiveMTLS := 0
	for _, entry := range entries {
		switch e := entry.(type) {
		case *api.ServiceConfigEntry:
			if e.MutualTLSMode == api.MutualTLSModePermissive {
				servicesInPermissiveMTLS++
			}
		case *api.ProxyConfigEntry:
			if msg := KindSpecificWarning(e); msg != "" {
				result = append(result, fmt.Sprintf("%s/%s: %s", e.GetKind(), e.GetName(), msg))
			}
		}
	}

	if servicesInPermissiveMTLS > 0 {
		msg := "Found %d service-default(s) with MutualTLSMode=permissive." +
			" Use `-filter 'MutualTLSMode == \"permissive\"'` to list service-defaults in permissive MutualTLSMode." +
			" This mode is insecure. We recommend setting this to MutualTLSMode=strict."
		result = append(result, fmt.Sprintf(msg, servicesInPermissiveMTLS))
	}
	return result
}
