---
layout: "docs"
page_title: "Commands: Keyring"
sidebar_current: "docs-commands-keyring"
---

# Consul Keyring

Command: `consul keyring`

The `keyring` command is used to examine and modify the encryption keys used in
Consul's [Gossip Pools](/docs/internals/gossip.html). It is capable of
distributing new encryption keys to the cluster, retiring old encryption keys,
and changing the keys used by the cluster to encrypt messages.

Because Consul utilizes multiple gossip pools, this command will only operate
against a server node for most operations.

Consul allows multiple encryption keys to be in use simultaneously. This is
intended to provide a transition state while the cluster converges. It is the
responsibility of the operator to ensure that only the required encryption keys
are installed on the cluster. You can ensure that a key is not installed using
the `-list` and `-remove` options.

All variations of the `keyring` command, unless otherwise specified below, will
return 0 if all nodes reply and there are no errors. If any node fails to reply
or reports failure, the exit code will be 1.

## Usage

Usage: `consul keyring [options]`

Only one actionable argument may be specified per run, including `-init`,
`-list`, `-install`, `-remove`, and `-use`.

The list of available flags are:

* `-init` - Creates the keyring file(s). This is useful to configure initial
  encryption keyrings, which can later be mutated using the other arguments in
  this command. This argument accepts an ASCII key, which can be generated using
  the [keygen command](/docs/commands/keygen.html).

  This operation can be run on both client and server nodes and requires no
  network connectivity.

	Returns 0 if the key is successfully configured, or 1 if there were any
	problems.

* `-install` - Install a new encryption key. This will broadcast the new key to
  all members in the cluster.

* `-use` - Change the primary encryption key, which is used to encrypt messages.
  The key must already be installed before this operation can succeed.

* `-remove` - Remove the given key from the cluster. This operation may only be
  performed on keys which are not currently the primary key.

* `-list` - List all keys currently in use within the cluster.

* `-wan` - Operate on the WAN keyring instead of the LAN keyring (default)

* `-rpc-addr` - RPC address of the Consul agent.
