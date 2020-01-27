---
layout: "docs"
page_title: "Commands: Config"
sidebar_current: "docs-commands-config"
---

# Consul Config

Command: `consul config`

The `config` command is used to interact with Consul's central configuration
system. It exposes commands for creating, updating, reading, and deleting
different kinds of config entries. See the
[agent configuration](/docs/agent/options.html#enable_central_service_config)
for more information on how to enable this functionality for centrally
configuring services and [configuration entries docs](/docs/agent/config_entries.html) for a description
of the configuration entries content.

## Usage

Usage: `consul config <subcommand>`

For the exact documentation for your Consul version, run `consul config -h` to view
the complete list of subcommands.

```text
Usage: consul config <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's centralized
  configuration system. Here are some simple examples, and more detailed
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
```

For more information, examples, and usage about a subcommand, click on the name
of the subcommand in the sidebar.