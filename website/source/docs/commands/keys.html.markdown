---
layout: "docs"
page_title: "Commands: Keys"
sidebar_current: "docs-commands-keys"
---

# Consul Keys

Command: `consul keys`

The `keys` command is used to examine and modify the encryption keys used in
Consul's [Gossip Pools](/docs/internals/gossip.html). It is capable of
distributing new encryption keys to the cluster, revoking old encryption keys,
and changing the key used by the cluster to encrypt messages.

Because Consul utilizes multiple gossip pools, this command will operate on only
a single pool at a time. The pool can be specified using the arguments
documented below.

Consul allows multiple encryption keys to be in use simultaneously. This is
intended to provide a transition state while the cluster converges. It is the
responsibility of the operator to ensure that only the required encryption keys
are installed on the cluster. You can ensure that a key is not installed using
the `-list` and `-remove` options.

By default, modifications made using this command will be persisted in the
Consul agent's data directory. This functionality can be altered via the
[Agent Configuration](/docs/agent/options.html).

All variations of the keys command will return 0 if all nodes reply and there
are no errors. If any node fails to reply or reports failure, the exit code will
be 1.

## Usage

Usage: `consul keys [options]`

Exactly one of `-list`, `-install`, `-remove`, or `-update` must be provided.

The list of available flags are:

* `-install` - Install a new encryption key. This will broadcast the new key to
  all members in the cluster.

* `-use` - Change the primary encryption key, which is used to encrypt messages.
  The key must already be installed before this operation can succeed.

* `-remove` - Remove the given key from the cluster. This operation may only be
  performed on keys which are not currently the primary key.

* `-list` - List all keys currently in use within the cluster.

* `-wan` - If talking with a server node, this flag can be used to operate on
  the WAN gossip layer. By default, this command operates on the LAN layer. More
  information about the different gossip layers can be found on the
  [gossip protocol](/docs/internals/gossip.html) page.

* `-rpc-addr` - RPC address of the Consul agent.
