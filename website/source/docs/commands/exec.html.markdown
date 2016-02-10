---
layout: "docs"
page_title: "Commands: Exec"
sidebar_current: "docs-commands-exec"
description: |-
  The exec command provides a mechanism for remote execution. For example, this can be used to run the `uptime` command across all machines providing the `web` service.
---

# Consul Exec

Command: `consul exec`

The `exec` command provides a mechanism for remote execution. For example,
this can be used to run the `uptime` command across all machines providing
the `web` service.

Remote execution works by specifying a job, which is stored in the KV store.
Agents are informed about the new job using the [event system](/docs/commands/event.html),
which propagates messages via the [gossip protocol](/docs/internals/gossip.html).
As a result, delivery is best-effort, and there is **no guarantee** of execution.

While events are purely gossip driven, remote execution relies on the KV store
as a message broker. As a result, the `exec` command will not be able to
properly function during a Consul outage.

**Verbose output warning:** use care to make sure that your command does not
produce a large volume of output. Writes to the KV store for this output go
through the Consul servers and the Raft consensus algorithm, so having a large
number of nodes in the cluster flow a large amount of data through the KV store
could make the cluster unavailable.

## Usage

Usage: `consul exec [options] [-|command...]`

The only required option is a command to execute. This is either given
as trailing arguments, or by specifying '-'; stdin will be read to
completion as a script to evaluate.

The list of available flags are:

* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

* `-datacenter` - Datacenter to query. Defaults to that of agent. In version
  0.4, that is the only supported value.

* `-prefix` - Key prefix in the KV store to use for storing request data.
  Defaults to "_rexec".

* `-node` - Regular expression to filter nodes which should evaluate the event.

* `-service` - Regular expression to filter to only nodes with matching services.

* `-tag` - Regular expression to filter to only nodes with a service that has
  a matching tag. This must be used with `-service`. As an example, you may
  do "-service mysql -tag slave".

* `-wait` - Specifies the period of time in which no agent's respond before considering
  the job finished. This is basically the quiescent time required to assume completion.
  This period is not a hard deadline, and the command will wait longer depending on
  various heuristics.

* `-wait-repl` - Period to wait after writing the job specification for replication.
  This is a heuristic value and enables agents to do a stale read of the job. Defaults
  to 200msec.

* `-verbose` - Enables verbose output.

* `-token` - The ACL token to use during requests. This token must have access
  to the prefix in the KV store as well as exec "write" access for the _rexec
  event. Defaults to that of the agent.
