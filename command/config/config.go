// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
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

const (
	// TODO(pglass): These warnings can go away when the UI provides visibility into
	// permissive mTLS settings (expected 1.17).
	WarningServiceDefaultsPermissiveMTLS = "MutualTLSMode=permissive is insecure. " +
		"Set to `strict` when your service no longer needs to accept non-mTLS " +
		"traffic. Check `tcp.permissive_public_listener` metrics in Envoy for " +
		"non-mTLS traffic. Refer to Consul documentation for more information."

	WarningProxyDefaultsPermissiveMTLS = "MutualTLSMode=permissive is insecure. " +
		"To keep your services secure, set MutualTLSMode to `strict` whenever possible " +
		"and override with service-defaults only if necessary. To check which " +
		"service-defaults are currently in permissive mode, run `consul config list " +
		"-kind service-defaults -filter 'MutualTLSMode = \"permissive\"'`."

	WarningMeshAllowEnablingPermissiveMutualTLS = "AllowEnablingPermissiveMutualTLS=true " +
		"allows insecure MutualTLSMode=permissive configurations in the proxy-defaults " +
		"and service-defaults config entries. You can set " +
		"AllowEnablingPermissiveMutualTLS=false at any time to disallow additional " +
		"permissive configurations. To list services in permissive mode, run `consul " +
		"config list -kind service-defaults -filter 'MutualTLSMode = \"permissive\"'`."
)

// KindSpecificWriteWarning returns a warning message for the given config
// entry write. Use this to inform the user of (un)recommended settings when
// they read or write config entries with the CLI.
//
// Do not return a warning on default/zero values. Because the config
// entry is parsed, we cannot distinguish between an absent field in the
// user-provided content and a zero value, so we'd end up warning on
// every invocation.
func KindSpecificWriteWarning(reqEntry api.ConfigEntry) string {
	switch req := reqEntry.(type) {
	case *api.ServiceConfigEntry:
		if req.MutualTLSMode == api.MutualTLSModePermissive {
			return WarningServiceDefaultsPermissiveMTLS
		}
	case *api.ProxyConfigEntry:
		if req.MutualTLSMode == api.MutualTLSModePermissive {
			return WarningProxyDefaultsPermissiveMTLS
		}
	case *api.MeshConfigEntry:
		if req.AllowEnablingPermissiveMutualTLS == true {
			return WarningMeshAllowEnablingPermissiveMutualTLS
		}
	}
	return ""
}
