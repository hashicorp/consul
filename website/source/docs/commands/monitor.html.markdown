---
layout: "docs"
page_title: "Commands: Monitor"
sidebar_current: "docs-commands-monitor"
description: |-
  The `monitor` command is used to connect and follow the logs of a running Consul agent. Monitor will show the recent logs and then continue to follow the logs, not exiting until interrupted or until the remote agent quits.
---

# Consul Monitor

Command: `consul monitor`

The `monitor` command is used to connect and follow the logs of a running
Consul agent. Monitor will show the recent logs and then continue to follow
the logs, not exiting until interrupted or until the remote agent quits.

The power of the monitor command is that it allows you to log the agent
at a relatively high log level (such as "warn"), but still access debug
logs and watch the debug logs if necessary.

## Usage

Usage: `consul monitor [options]`

The command-line flags are all optional. The list of available flags are:

* `-log-level` - The log level of the messages to show. By default this
  is "info". This log level can be more verbose than what the agent is
  configured to run at. Available log levels are "trace", "debug", "info",
  "warn", and "err".

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command checks the
  CONSUL_RPC_ADDR env variable. If this isn't set, the default RPC 
  address will be set to "127.0.0.1:8400". 
