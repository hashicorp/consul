---
layout: "docs"
page_title: "Commands: Watch"
sidebar_current: "docs-commands-watch"
description: |-
  The `watch` command provides a mechanism to watch for changes in a particular data view (list of nodes, service members, key value, etc) and to invoke a process with the latest values of the view. If no process is specified, the current values are dumped to stdout which can be a useful way to inspect data in Consul.
---

# Consul Watch

Command: `consul watch`

The `watch` command provides a mechanism to watch for changes in a particular
data view (list of nodes, service members, key value, etc) and to invoke
a process with the latest values of the view. If no process is specified,
the current values are dumped to stdout which can be a useful way to inspect
data in Consul.

There is more [documentation on watches here](/docs/agent/watches.html).

## Usage

Usage: `consul watch [options] [child...]`

The only required option is `-type` which specifies the particular
data view. Depending on the type, various options may be required
or optionally provided. There is more documentation on watch
[specifications here](/docs/agent/watches.html).

The list of available flags are:

* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

* `-datacenter` - Datacenter to query. Defaults to that of agent.

* `-token` - ACL token to use. Defaults to that of agent.

* `-key` - Key to watch. Only for `key` type.

* `-name`- Event name to watch. Only for `event` type.

* `-passingonly=[true|false]` - Should only passing entries be returned. Default false.
  only for `service` type.

* `-prefix` - Key prefix to watch. Only for `keyprefix` type.

* `-service` - Service to watch. Required for `service` type, optional for `checks` type.

* `-state` - Check state to filter on. Optional for `checks` type.

* `-tag` - Service tag to filter on. Optional for `service` type.

* `-type` - Watch type. Required, one of "key", "keyprefix", "services",
  "nodes", "services", "checks", or "event".

