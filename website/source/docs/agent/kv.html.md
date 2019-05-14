---
layout: "docs"
page_title: "Consul KV"
sidebar_current: "docs-agent-kv"
description: |- 
    Consul KV is a core feature of Consul and is installed with the Consul agent.
---

# Consul KV 

Consul KV is a core feature of Consul and is installed with the Consul agent.
Once installed with the agent, it will have sane defaults. Consul KV allows
users to store indexed objects, though its main uses are storing configuration
parameters and metadata. Please note that it is a simple KV store and is not
intended to be a full featured datastore (such as DynamoDB) but has some
similarities to one. 

The Consul KV datastore is located on the servers, but can be accessed by any
agent (client or server). The natively integrated [RPC
functionality](/docs/internals/architecture.html) allows clients to forward
requests to servers, including key/value reads and writes. Part of Consul’s
core design allows data to be replicated automatically across all the servers.
Having a quorum of servers will decrease the risk of data loss if an outage
occurs.

## Accessing the KV store

The KV store can be accessed by the [consul kv CLI
subcommands](/docs/commands/kv.html), [HTTP API](/api/kv.html), and Consul UI.
To restrict access, enable and configure
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).
Once the ACL system has been bootstrapped, users and services, will need a
valid token with KV [privileges](/docs/agent/acl-rules.html#key-value-rules) to
access the the data store, this includes even reads.  We recommend creating a
token with limited privileges, for example, you could create a token with write
privileges on one key for developers to update the value related to their
application.

The datastore itself is located on the Consul servers in the [data
directory](/docs/agent/options.html#_data_dir). To ensure data is not lost in
the event of a complete outage, use the [`consul
snapshot`](/docs/commands/snapshot/restore.html) feature to backup the data. 

## Using Consul KV

Objects are opaque to Consul, meaning there are no restrictions on the type of
object stored in a key/value entry. The main restriction on an object is size -
the maximum is 512 KB. Due to the maximum object size and main use cases, you
should not need extra storage; the general [sizing
recommendations](/docs/commands/snapshot/restore.html) are usually sufficient. 

Keys, like objects are not restricted by type and can include any character.
However, we recommend using URL-safe chars - `[a-zA-Z0-9-_]`  with the
exception of  `/`, which can be used to help organize data. Note, `/` will be
treated like any other character and is not fixed to the file system. Meaning,
including `/` in a key does not fix it to a directory structure. This model is
similar to Amazon S3 buckets. However, `/`  is still useful for organizing data
and when recursively searching within the data store. We also recommend that
you avoid the use of  `*`, `?`, `'`, and `%` because they can cause issues when
using the API and in shell scripts. 

If you have not used Consul KV, check out this [Getting Started
guide](https://learn.hashicorp.com/consul/getting-started/kv) on HashiCorp
Learn. 

## Extending Consul KV

### Consul Template

If you plan to use Consul KV as part of your configuration management process
review the [Consul
Template](https://learn.hashicorp.com/consul/developer-configuration/consul-template)
guide on how to update configuration based on value updates in the KV. Consul
Template is based on Go Templates and allows for a series of scripted actions
to be initiated on value changes to a Consul key. 

### Watches

Consul KV can also be extended with the use of watches.
[Watches](/docs/agent/watches.html) are a way to monitor data for updates. When
an update is detected, an external handler is invoked. To use watches with the
KV store the [key](/docs/agent/watches.html#key) watch type should be used. 

### Consul Sessions

Consul sessions can be used to build distributed locks with Consul KV. Sessions
act as a binding layer between nodes, health checks, and key/value data. The KV
API supports an `acquire` and `release` operation. The `acquire` operation acts
like a Check-And-Set operation. On success, there is a key update and an
increment to the `LockIndex` and the session value is updated to reflect the
session holding the lock. Review the session documentation for more information
on the [integration](/docs/internals/sessions.html#k-v-integration)

### Vault

If you plan to use Consul KV as a backend for Vault, please review [this
guide](https://learn.hashicorp.com/vault/operations/ops-vault-ha-consul).
