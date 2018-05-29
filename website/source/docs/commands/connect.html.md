---
layout: "docs"
page_title: "Commands: Connect"
sidebar_current: "docs-commands-connect"
---

# Consul Connect

Command: `consul connect`

The `connect` command is used to interact with Connect
[Connect](/docs/connect/intentions.html) subsystems. It exposes commands for
running the built-in mTLS proxy and viewing/updating the Certificate Authority
(CA) configuration. This command is available in Consul 1.2 and later.

## Usage

Usage: `consul connect <subcommand>`

For the exact documentation for your Consul version, run `consul connect -h` to view
the complete list of subcommands.

```text
Usage: consul connect <subcommand> [options] [args]

  This command has subcommands for interacting with Consul Connect.

  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Run the built-in Connect mTLS proxy

      $ consul connect proxy

  For more examples, ask for subcommand help or view the documentation.

Subcommands:
    ca       Interact with the Consul Connect Certificate Authority (CA)
    proxy    Runs a Consul Connect proxy
```

For more information, examples, and usage about a subcommand, click on the name
of the subcommand in the sidebar.