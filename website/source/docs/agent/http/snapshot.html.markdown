---
layout: "docs"
page_title: "Consul Snapshots (HTTP)"
sidebar_current: "docs-agent-http-snapshots"
description: >
  The Snapshot endpoints are used to save and restore Consul's server state for disaster recovery.
---

# Snapshot HTTP Endpoint

The Snapshot endpoints are used to save and restore the state of the Consul
servers for disaster recovery. Snapshots include all state managed by Consul's
Raft [consensus protocol](/docs/internals/consensus.html), including:

* Key/Value Entries
* Service Catalog
* Prepared Queries
* Sessions
* ACLs

These endpoints, available in Consul 0.7.1 and later, allow for atomic,
point-in-time snapshots of the above data to be obtained in a format that can be
saved externally. These snapshots can then be used to restore the server state
into a fresh cluster of Consul servers in the event of a disaster.

The following endpoints are supported:

* [`/v1/snapshot`](#snapshot): Save and restore Consul server state

These endpoints do not support blocking queries and always use the consistent
mode for reads. Requests are always forwarded internally to the cluster leader.

The endpoints support the use of ACL Tokens. Because snapshots contain all
server state, including ACLs, a management token is required to perform snapshot
operations is ACLs are enabled.

-> Snapshot operations are not available for servers running in
   [dev mode](/docs/agent/options.html#_dev).

### <a name="snapshot"></a> /v1/snapshot

The snapshot endpoint supports the `GET` and `PUT` methods.

#### GET Method

When using the `GET` method, Consul will perform an atomic, point-in-time
snapshot of the Consul server state.

Snapshots are exposed as zip archives which internally contain the Raft metadata
required to restore, as well as a binary serialized version of the Consul server
state. In addition to the CRC provided by zip, the contents are covered internally
by SHA-256 hashes. These hashes are verified during snapshot restore operations.
The structure of the zip archive is internal to Consul and not intended to be used
other than for restore operations. In particular, the zip archives are not designed
to be edited.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with management
privileges.

The return code is 200 on success, and the snapshot will be returned in the body
as a zip archive.

#### PUT Method

When using the `PUT` method, Consul will atomically restore a point-in-time
snapshot of the Consul server state.

Restores involve a potentially dangerous low-level Raft operation that is not
designed to handle server failures during a restore. This operation is primarily
intended to be used when recovering from a disaster, restoring into a fresh
cluster of Consul servers.

By default, the datacenter of the agent is targeted; however, the `dc` can be
provided using the "?dc=" query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with management
privileges.

The body of the request should a snapshot zip archive as given by the `GET` method.

The return code is 200 on success.
