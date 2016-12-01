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

Available in Consul 0.7.1 and later, these endpoints allow for atomic,
point-in-time snapshots of the above data in a format that can be saved
externally. Snapshots can then be used to restore the server state into a fresh
cluster of Consul servers in the event of a disaster.

The following endpoints are supported:

* [`/v1/snapshot`](#snapshot): Save and restore Consul server state

These endpoints do not support blocking queries. Saving snapshots uses the
consistent mode by default and stale mode is supported.

The endpoints support the use of ACL Tokens. Because snapshots contain all
server state, including ACLs, a management token is required to perform snapshot
operations if ACLs are enabled.

### <a name="snapshot"></a> /v1/snapshot

The snapshot endpoint supports the `GET` and `PUT` methods.

#### GET Method

When using the `GET` method, Consul will perform an atomic, point-in-time
snapshot of the Consul server state.

Snapshots are exposed as gzipped tar archives which internally contain the Raft
metadata required to restore, as well as a binary serialized version of the Consul
server state. The contents are covered internally by SHA-256 hashes. These hashes
are verified during snapshot restore operations. The structure of the archive is
internal to Consul and not intended to be used other than for restore operations.
In particular, the archives are not designed to be modified before a restore.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

By default, snapshots use consistent mode which means the request is internally
forwarded to the cluster leader, and leadership is checked before performing the
snapshot. If `stale` is specified using the `?stale` query parameter, then any
server can handle the request and the results may be arbitrarily stale. To support
bounding the acceptable staleness of snapshots, responses provide the `X-Consul-LastContact`
header containing the time in milliseconds that a server was last contacted by
the leader node. The `X-Consul-KnownLeader` header also indicates if there is a
known leader. These can be used by clients to gauge the staleness of a snapshot
and take appropriate action. The stale mode is particularly useful from taking a
snapshot of a cluster in a failed state with no current leader.

If ACLs are enabled, the client will need to supply an ACL Token with management
privileges.

The return code is 200 on success, and the snapshot will be returned in the body
as a gzipped tar archive. In addition to the stale-related headers described above,
the `X-Consul-Index` header will also be set to the index at which the snapshot took
place.

#### PUT Method

When using the `PUT` method, Consul will atomically restore a point-in-time
snapshot of the Consul server state.

Restores involve a potentially dangerous low-level Raft operation that is not
designed to handle server failures during a restore. This operation is primarily
intended to be used when recovering from a disaster, restoring into a fresh
cluster of Consul servers.

By default, the datacenter of the agent is targeted; however, the `dc` can be
provided using the `?dc=` query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with management
privileges.

The body of the request should be a snapshot archive returned from a previous call
to the `GET` method.

The return code is 200 on success.
