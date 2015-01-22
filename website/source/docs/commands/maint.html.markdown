---
layout: "docs"
page_title: "Commands: Maint"
sidebar_current: "docs-commands-maint"
description: >
  The `maint` command provides control of both service and node maintenance mode
---

# Consul Maint

Command: `consul maint`

The `maint` command provides control of both service and node maintenance mode.
Using the command, it is possible to mark a service provided by a node or the
node as a whole as "under maintenance". In this mode of operation, the service
or node will not appear in DNS query results, or API results. This effectively
takes the service or node out of the pool of available "healthy" nodes.

Under the hood, maintenance mode is activated by registering a health check in
critical status against a node or service, and deactivated by deregistering the
health check.

## Usage

Usage: `consul maint [options]`

All of the command line arguments are optional.

The list of available flags are:

* `-enable` - Enable maintenance mode on a given service or node. If
  combined with the `-service` flag, we operate on a specific service ID.
  Otherwise, node maintenance mode is enabled.

* `-disable` - Disable maintenance mode on a given service or node. If
  combined with the `-service` flag, we operate on a specific service ID.
  Otherwise, node maintenance mode is disabled.

* `-reason` - An optional reason for placing the node or service into
  maintenance mode. If provided, this reason will be visible in the newly-
  registered critical check's "Notes" field.

* `-service` - An optional service ID to control node maintenance mode for. By
  providing this flag, the `-enable` and `-disable` flags functionality is
  modified to operate on the given service ID.

* `-token` - ACL token to use. Defaults to that of agent.

* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

## List mode

If neither `-enable` nor `-disable` are passed, the `maint` command will
switch to "list mode", displaying any current maintenances. This may return
blank if nothing is currently under maintenance. The output will look like:

```
$ consul maint
Node:
  Name:   node1.local
  Reason: This node is broken.

Service:
  ID:     redis
  Reason: Redis is currently offline.
```
