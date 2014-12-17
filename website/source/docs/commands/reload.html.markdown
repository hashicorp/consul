---
layout: "docs"
page_title: "Commands: Reload"
sidebar_current: "docs-commands-reload"
description: |-
  The `reload` command triggers a reload of configuration files for the agent.
---

# Consul Reload

Command: `consul reload`

The `reload` command triggers a reload of configuration files for the agent.

The `SIGHUP` signal is usually used to trigger a reload of configurations,
but in some cases it may be more convenient to trigger the CLI instead.

This command operates the same as the signal, meaning that it will trigger
a reload, but does not wait for the reload to complete. Any errors with the
reload will be present in the agent logs and not in the output of this command.

**NOTE**

Not all configuration options are reloadable. See the
[Reloadable Configuration](/docs/agent/options.html#reloadable-configuration)
section on the agent options page for details on which options are supported.

## Usage

Usage: `consul reload`

The command-line flags are all optional. The list of available flags are:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command checks the
  CONSUL_RPC_ADDR env variable. If this isn't set, the default RPC 
  address will be set to "127.0.0.1:8400".

