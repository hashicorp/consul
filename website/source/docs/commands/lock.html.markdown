---
layout: "docs"
page_title: "Commands: Lock"
sidebar_current: "docs-commands-lock"
description: |-
  The lock command provides a mechanism for leader election, mutual exclusion, or worker pools. For example, this can be used to ensure a maximum number of services running at once across a cluster.
---

# Consul Lock

Command: `consul lock`

The `lock` command provides a mechanism for simple distributed locking.
A lock (or semaphore) is created at a given prefix in the Key/Value store,
and only when held, is a child process invoked. If the lock is lost or
communication is disrupted, the child process is terminated.

The number of lock holders is configurable with the `-n` flag. By default,
a single holder is allowed, and a lock is used for mutual exclusion. This
uses the [leader election algorithm](/docs/guides/leader-election.html).

If the lock holder count is more than one, then a semaphore is used instead.
A semaphore allows more than a single holder, but this is less efficient than
a simple lock. This follows the [semaphore algorithm](/docs/guides/semaphore.html).

All locks using the same prefix must agree on the value of `-n`. If conflicting
values of `-n` are provided, an error will be returned.

An example use case is for highly-available N+1 deployments. In these
cases, if N instances of a service are required, N+1 are deployed and use
consul lock with `-n=N` to ensure only N instances are running. For singleton
services, a hot standby waits until the current leader fails to take over.

## Usage

Usage: `consul lock [options] prefix child...`

The only required options are the key prefix and the command to execute.
The prefix must be writable. The child is invoked only when the lock is held,
and the `CONSUL_LOCK_HELD` environment variable will be set to `true`.

If the lock is lost, communication is disrupted, or the parent process
interrupted, the child process will receive a `SIGTERM`. After a grace period
of 5 seconds, a `SIGKILL` will be used to force termination. For Consul agents
on Windows, the child process is always terminated with a `SIGKILL`, since
Windows has no POSIX compatible notion for `SIGTERM`.

The list of available flags are:

* `-http-addr` - Address to the HTTP server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:8500" which is the default HTTP address of a Consul agent.

* `-n` - Optional, limit of lock holders. Defaults to 1. The underlying
  implementation switches from a lock to a semaphore when increased past
  one. All locks on the same prefix must use the same value.

* `-name` - Optional name to associate with the underlying session.
  If not provided, one is generated based on the child command.

* `-token` - ACL token to use. Defaults to that of agent.

* `-pass-stdin` - Pass stdin to child process.

* `-try` - Attempt to acquire the lock up to the given timeout. The timeout is a
  positive decimal number, with unit suffix, such as "500ms". Valid time units
  are "ns", "us" (or "µs"), "ms", "s", "m", "h".

* `-monitor-retry` - Retry up to this number of times if Consul returns a 500 error
   while monitoring the lock. This allows riding out brief periods of unavailability
   without causing leader elections, but increases the amount of time required
   to detect a lost lock in some cases. Defaults to 3, with a 1s wait between retries.
   Set to 0 to disable.

* `-verbose` - Enables verbose output.

