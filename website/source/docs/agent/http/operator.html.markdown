---
layout: "docs"
page_title: "Operator (HTTP)"
sidebar_current: "docs-agent-http-operator"
description: >
  The operator endpoint provides cluster-level tools for Consul operators.
---

# Operator HTTP Endpoint

The Operator endpoint provides cluster-level tools for Consul operators, such
as interacting with the Raft subsystem. This was added in Consul 0.7.

~> Use this interface with extreme caution, as improper use could lead to a Consul
   outage and even loss of data.

If ACLs are enabled then a token with operator privileges may be required in
order to use this interface. See the [ACL](/docs/internals/acl.html#operator)
internals guide for more information.

See the [Outage Recovery](/docs/guides/outage.html) guide for some examples of how
these capabilities are used. For a CLI to perform these operations manually, please
see the documentation for the [`consul operator`](/docs/commands/operator.html)
command.

The following types of endpoints are supported:

* [Autopilot](#autopilot): Automatically manage Consul servers
* [Keyring](#keyring): Manage gossip encryption keyring
* [Network Areas](#network-areas): Manage network areas (Enterprise-only)
* [Raft](#raft): Manage Raft consensus subsystem

Not all endpoints support blocking queries and all consistency modes,
see details in the sections below.

The operator endpoints support the use of ACL Tokens. See the
[ACL](/docs/internals/acl.html#operator) internals guide for more information.

## Autopilot

Autopilot is a set of new features added in Consul 0.8 to allow for automatic
operator-friendly management of Consul servers. Please see the
[Autopilot Guide](/docs/guides/autopilot.html) for more details.

The following endpoints are supported:

* [`/v1/operator/autopilot/configuration`](#autopilot-configuration): Read or update Autopilot configuration
* [`/v1/operator/autopilot/health`](#autopilot-health): Read server health as determined by Autopilot

### <a name="autopilot-configuration"></a> /v1/operator/autopilot/configuration

Available in Consul 0.8.0 and later, the autopilot configuration endpoint supports the
`GET` and `PUT` methods.

This endpoint supports the use of ACL tokens using either the `X-CONSUL-TOKEN`
header or the `?token=` query parameter.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

#### GET Method

When using the `GET` method, the request will be forwarded to the cluster
leader to retrieve its latest Autopilot configuration.

If the cluster doesn't currently have a leader an error will be returned. You
can use the `?stale` query parameter to read the Raft configuration from any
of the Consul servers.

If ACLs are enabled, the client will need to supply an ACL Token with
[`operator`](/docs/internals/acl.html#operator) read privileges.

A JSON body is returned that looks like this:

```javascript
{
    "CleanupDeadServers": true,
    "LastContactThreshold": "200ms",
    "MaxTrailingLogs": 250,
    "ServerStabilizationTime": "10s",
    "RedundancyZoneTag": "",
    "DisableUpgradeMigration": false,
    "CreateIndex": 4,
    "ModifyIndex": 4
}
```

For more information about the Autopilot configuration options, see the agent configuration section
[here](/docs/agent/options.html#autopilot).

#### PUT Method

Using the `PUT` method, this endpoint will update the Autopilot configuration
of the cluster.

The `?cas=<index>` can optionally be specified to update the configuration as a
Check-And-Set operation. The update will only happen if the given index matches
the `ModifyIndex` of the configuration at the time of writing.

If ACLs are enabled, the client will need to supply an ACL Token with
[`operator`](/docs/internals/acl.html#operator) write privileges.

The `PUT` method expects a JSON request body to be submitted. The request
body must look like:

```javascript
{
    "CleanupDeadServers": true,
    "LastContactThreshold": "200ms",
    "MaxTrailingLogs": 250,
    "ServerStabilizationTime": "10s",
    "RedundancyZoneTag": "",
    "DisableUpgradeMigration": false,
    "CreateIndex": 4,
    "ModifyIndex": 4
}
```

For more information about the Autopilot configuration options, see the agent configuration section
[here](/docs/agent/options.html#autopilot).

The return code will indicate success or failure.

### <a name="autopilot-health"></a> /v1/operator/autopilot/health

Available in Consul 0.8.0 and later, the autopilot health endpoint supports the
`GET` method.

This endpoint supports the use of ACL tokens using either the `X-CONSUL-TOKEN`
header or the `?token=` query parameter.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

#### GET Method

When using the `GET` method, the request will be forwarded to the cluster
leader to retrieve its latest Autopilot configuration.

If ACLs are enabled, the client will need to supply an ACL Token with
[`operator`](/docs/internals/acl.html#operator) read privileges.

A JSON body is returned that looks like this:

```javascript
{
    "Healthy": true,
    "FailureTolerance": 0,
    "Servers": [
        {
            "ID": "e349749b-3303-3ddf-959c-b5885a0e1f6e",
            "Name": "node1",
            "Address": "127.0.0.1:8300",
            "SerfStatus": "alive",
            "Version": "0.7.4",
            "Leader": true,
            "LastContact": "0s",
            "LastTerm": 2,
            "LastIndex": 46,
            "Healthy": true,
            "Voter": true,
            "StableSince": "2017-03-06T22:07:51Z"
        },
        {
            "ID": "e36ee410-cc3c-0a0c-c724-63817ab30303",
            "Name": "node2",
            "Address": "127.0.0.1:8205",
            "SerfStatus": "alive",
            "Version": "0.7.4",
            "Leader": false,
            "LastContact": "27.291304ms",
            "LastTerm": 2,
            "LastIndex": 46,
            "Healthy": true,
            "Voter": false,
            "StableSince": "2017-03-06T22:18:26Z"
        }
    ]
}
```

`Healthy` is whether all the servers are currently heathly.

`FailureTolerance` is the number of redundant healthy servers that could be fail
without causing an outage (this would be 2 in a healthy cluster of 5 servers).

The `Servers` list holds detailed health information on each server:

- `ID` is the Raft ID of the server.

- `Name` is the node name of the server.

- `Address` is the address of the server.

- `SerfStatus` is the SerfHealth check status for the server.

- `Version` is the Consul version of the server.

- `Leader` is whether this server is currently the leader.

- `LastContact` is the time elapsed since this server's last contact with the leader.

- `LastTerm` is the server's last known Raft leader term.

- `LastIndex` is the index of the server's last committed Raft log entry.

- `Healthy` is whether the server is healthy according to the current Autopilot configuration.

- `Voter` is whether the server is a voting member of the Raft cluster.

- `StableSince` is the time this server has been in its current `Healthy` state.

## Keyring

The keyring endpoint allows management of the gossip encryption keyring. See
the [Gossip Protocol Guide](/docs/internals/gossip.html) for more details on the
gossip protocol and its use.

The following endpoint is supported:

* [`/v1/operator/keyring`](#keyring-endpoint): Operate on the gossip keyring

### <a name="keyring-endpoint"></a> /v1/operator/keyring

Available in Consul 0.7.2 and later, the keyring endpoint supports the
`GET`, `POST`, `PUT` and `DELETE` methods.

This endpoint supports the use of ACL tokens using either the `X-CONSUL-TOKEN`
header or the `?token=` query parameter.

Added in Consul 0.7.4, this endpoint supports the `?relay-factor=` query parameter.
See the [Keyring Command](/docs/commands/keyring.html#_relay_factor) for more details.

#### GET Method

Using the `GET` method, this endpoint will list the gossip encryption keys
installed on both the WAN and LAN rings of every known datacenter. There is more
information on gossip encryption available
[here](/docs/agent/encryption.html#gossip-encryption).

If ACLs are enabled, the client will need to supply an ACL Token with
[`keyring`](/docs/internals/acl.html#keyring) read privileges.

A JSON body is returned that looks like this:

```javascript
[
    {
        "WAN": true,
        "Datacenter": "dc1",
        "Keys": {
            "0eK8RjnsGC/+I1fJErQsBA==": 1,
            "G/3/L4yOw3e5T7NTvuRi9g==": 1,
            "z90lFx3sZZLtTOkutXcwYg==": 1
        },
        "NumNodes": 1
    },
    {
        "WAN": false,
        "Datacenter": "dc1",
        "Keys": {
            "0eK8RjnsGC/+I1fJErQsBA==": 1,
            "G/3/L4yOw3e5T7NTvuRi9g==": 1,
            "z90lFx3sZZLtTOkutXcwYg==": 1
        },
        "NumNodes": 1
    }
]
```

`WAN` is true if the block refers to the WAN ring of that datacenter (rather than
 LAN).

`Datacenter` is the datacenter the block refers to.

`Keys` is a map of each gossip key to the number of nodes it's currently installed
 on.

`NumNodes` is the total number of nodes in the datacenter.

#### POST Method

Using the `POST` method, this endpoint will install a new gossip encryption key
into the cluster. There is more information on gossip encryption available
[here](/docs/agent/encryption.html#gossip-encryption).

The `POST` method expects a JSON request body to be submitted. The request
body must look like:

```javascript
{
  "Key": "3lg9DxVfKNzI8O+IQ5Ek+Q=="
}
```

The `Key` field is mandatory and provides the encryption key to install into the
cluster.

If ACLs are enabled, the client will need to supply an ACL Token with
[`keyring`](/docs/internals/acl.html#keyring) write privileges.

The return code will indicate success or failure.

#### PUT Method

Using the `PUT` method, this endpoint will change the primary gossip encryption
key. The key must already be installed before this operation can succeed. There
is more information on gossip encryption available
[here](/docs/agent/encryption.html#gossip-encryption).

The `PUT` method expects a JSON request body to be submitted. The request
body must look like:

```javascript
{
 "Key": "3lg9DxVfKNzI8O+IQ5Ek+Q=="
}
```

The `Key` field is mandatory and provides the primary encryption key to begin
using.

If ACLs are enabled, the client will need to supply an ACL Token with
[`keyring`](/docs/internals/acl.html#keyring) write privileges.

The return code will indicate success or failure.

#### DELETE Method

Using the `DELETE` method, this endpoint will remove a gossip encryption key from
the cluster. This operation may only be performed on keys which are not currently
the primary key. There is more information on gossip encryption available
[here](/docs/agent/encryption.html#gossip-encryption).

The `DELETE` method expects a JSON request body to be submitted. The request
body must look like:

```javascript
{
 "Key": "3lg9DxVfKNzI8O+IQ5Ek+Q=="
}
```

The `Key` field is mandatory and provides the encryption key to remove from the
cluster.

If ACLs are enabled, the client will need to supply an ACL Token with
[`keyring`](/docs/internals/acl.html#keyring) write privileges.

The return code will indicate success or failure.

## Network Areas

~> The network area functionality described here is available only in
   [Consul Enterprise](https://www.hashicorp.com/consul.html) version 0.8.0 and later.

Consul Enterprise version supports network areas, which are operator-defined relationships
between servers in two different Consul datacenters.

Unlike Consul's WAN feature, network areas use just the server RPC port for communication,
and relationships can be made between independent pairs of datacenters, so not all servers
need to be fully connected. This allows for complex topologies among Consul datacenters like
hub/spoke and more general trees.

See the [Network Areas Guide](/docs/guides/areas.html) for more details.

The following endpoints are supported:

* [`/v1/operator/area`](#area-general): Create a new area or list areas
* [`/v1/operator/area/<id>`](#area-specific): Delete an area
* [`/v1/operator/area/<id>/join`](#area-join): Join Consul servers into an area
* [`/v1/operator/area/<id>/members`](#area-members): List Consul servers in an area

### <a name="area-general"></a> /v1/operator/area

The general network area endpoint supports the `POST` and `GET` methods.

#### POST Method

When using the `POST` method, Consul will create a new network area and return
its ID if it is created successfully.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with `operator`
write privileges.

The create operation expects a JSON request body that defines the network area,
like this example:

```javascript
{
  "PeerDatacenter": "dc2",
  "RetryJoin": [ "10.1.2.3", "10.1.2.4", "10.1.2.5" ]
}
```

`PeerDatacenter` is required and is the name of the Consul datacenter that will
be joined the Consul servers in the current datacenter to form the area. Only
one area is allowed for each possible `PeerDatacenter`, and a datacenter cannot
form an area with itself.

`RetryJoin` is a list of Consul servers to attempt to join. Servers can be given
as `IP`, `IP:port`, `hostname`, or `hostname:port`. Consul will spawn a background
task that tries to periodically join the servers in this list and will run until a
join succeeds. If this list isn't supplied, joining can be done with a call to the
[join endpoint](#area-join) once the network area is created.

The return code is 200 on success and the ID of the created network area is returned
in a JSON body:

```javascript
{
  "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05"
}
```

#### GET Method

When using the `GET` method, Consul will provide a listing of all network areas.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter. This endpoint supports blocking
queries and all consistency modes.

If ACLs are enabled, the client will need to supply an ACL Token with `operator`
read privileges.

This returns a JSON list of network areas, which looks like:

```javascript
[
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "PeerDatacenter": "dc2",
    "RetryJoin": [ "10.1.2.3", "10.1.2.4", "10.1.2.5" ]
  },
  ...
]
```

### <a name="area-specific"></a> /v1/operator/area/\<id\>

The specific network area endpoint supports the `GET` and `DELETE` methods.

#### GET Method

When using the `GET` method, Consul will provide a listing of a specific
network area.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter. This endpoint supports blocking
queries and all consistency modes.

If ACLs are enabled, the client will need to supply an ACL Token with `operator`
read privileges.

This returns a JSON list with a single network area, which looks like:

```javascript
[
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "PeerDatacenter": "dc2",
    "RetryJoin": [ "10.1.2.3", "10.1.2.4", "10.1.2.5" ]
  }
]
```

#### Delete Method

When using the `DELETE` method, Consul will delete a specific network area.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with `operator`
write privileges.

### <a name="area-join"></a> /v1/operator/area/\<id\>/join

The network area join endpoint supports the `PUT` method.

#### PUT Method

When using the `PUT` method, Consul will attempt to join the given Consul servers
into a specific network area.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with `operator`
write privileges.

The create operation expects a JSON request body with a list of Consul servers to
join, like this example:

```javascript
[ "10.1.2.3", "10.1.2.4", "10.1.2.5" ]
```

Servers can be given as `IP`, `IP:port`, `hostname`, or `hostname:port`.

The return code is 200 on success a JSON response will be returned with a summary
of the join results:

```javascript
[
  {
    "Address": "10.1.2.3",
    "Joined": true,
    "Error", ""
  },
  {
    "Address": "10.1.2.4",
    "Joined": true,
    "Error", ""
  },
  {
    "Address": "10.1.2.5",
    "Joined": true,
    "Error", ""
  }
]
```

`Address` has the address requested to join.

`Joined` will be `true` if the Consul server at the given address was successfully
joined into the network area. Otherwise, this will be `false` and `Error` will have
a human-readable message about why the join didn't succeed.

### <a name="area-members"></a> /v1/operator/area/\<id\>/members

The network area members endpoint supports the `GET` method.

#### GET Method

When using the `GET` method, Consul will provide a listing of the Consul servers
present in a specific network area.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with `operator`
read privileges.

This returns a JSON list with details about the Consul servers present in the network
area, like this:

```javascript
[
  {
    "ID": "afc5d95c-1eee-4b46-b85b-0efe4c76dd48",
    "Name": "node-2.dc1",
    "Addr": "127.0.0.2",
    "Port": 8300,
    "Datacenter": "dc1",
    "Role": "server",
    "Build": "0.8.0",
    "Protocol": 2,
    "Status": "alive",
    "RTT": 256478
  },
  ...
]
```

`ID` is the node ID of the server.

`Name` is the node name of the server, with its datacenter appended.

`Addr` is the IP address of the node.

`Port` is the server RPC port of the node.

`Datacenter` is the node's Consul datacenter.

`Role` is always "server" since only Consul servers can participate in network
areas.

`Build` has the Consul version running on the node.

`Protocol` is the [protocol version](/docs/upgrading.html#protocol-versions) being
spoken by the node.

`Status` is the current health status of the node, as determined by the network
area distributed failure detector. This will be "alive", "leaving", "left", or
"failed". A "failed" status means that other servers are not able to probe this
server over its server RPC interface.

`RTT` is an estimated network round trip time from the server answering the query
to the given server, in nanoseconds. This is computed using
[network coordinates](/docs/internals/coordinates.html).

## Raft

The Raft endpoint provides tools for Management of Raft the consensus subsystem
and cluster quorum. See the [Consensus Protocol Guide](/docs/internals/consensus.html)
for more information about Raft consensus protocol and its use.

The following endpoints are supported:

* [`/v1/operator/raft/configuration`](#raft-configuration): Inspect the Raft configuration
* [`/v1/operator/raft/peer`](#raft-peer): Remove a server from the Raft configuration

### <a name="raft-configuration"></a> /v1/operator/raft/configuration

The Raft configuration endpoint supports the `GET` method.

#### GET Method

When using the `GET` method, the request will be forwarded to the cluster
leader to retrieve its latest Raft peer configuration.

If the cluster doesn't currently have a leader an error will be returned. You
can use the `?stale` query parameter to read the Raft configuration from any
of the Consul servers.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the `?dc=` query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with
[`operator`](/docs/internals/acl.html#operator) read privileges.

A JSON body is returned that looks like this:

```javascript
{
  "Servers": [
    {
      "ID": "127.0.0.1:8300",
      "Node": "alice",
      "Address": "127.0.0.1:8300",
      "Leader": true,
      "Voter": true
    },
    {
      "ID": "127.0.0.2:8300",
      "Node": "bob",
      "Address": "127.0.0.2:8300",
      "Leader": false,
      "Voter": true
    },
    {
      "ID": "127.0.0.3:8300",
      "Node": "carol",
      "Address": "127.0.0.3:8300",
      "Leader": false,
      "Voter": true
    }
  ],
  "Index": 22
}
```

The `Servers` array has information about the servers in the Raft peer
configuration:

`ID` is the ID of the server. This is the same as the `Address` in Consul 0.7
but may be upgraded to a GUID in a future version of Consul.

`Node` is the node name of the server, as known to Consul, or "(unknown)" if
the node is stale and not known.

`Address` is the IP:port for the server.

`Leader` is either "true" or "false" depending on the server's role in the
Raft configuration.

`Voter` is "true" or "false", indicating if the server has a vote in the Raft
configuration. Future versions of Consul may add support for non-voting servers.

The `Index` value is the Raft corresponding to this configuration. The latest configuration may not yet be committed if changes are in flight.

### <a name="raft-peer"></a> /v1/operator/raft/peer

The Raft peer endpoint supports the `DELETE` method.

#### DELETE Method

Using the `DELETE` method, this endpoint will remove the Consul server with
given address from the Raft configuration.

There are rare cases where a peer may be left behind in the Raft configuration
even though the server is no longer present and known to the cluster. This
endpoint can be used to remove the failed server so that it is no longer
affects the Raft quorum.

An `?address=` query parameter is required and should be set to the
`IP:port` for the server to remove. The port number is usually 8300, unless
configured otherwise. Nothing is required in the body of the request.

By default, the datacenter of the agent is targeted; however, the `dc` can be
provided using the `?dc=` query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with
[`operator`](/docs/internals/acl.html#operator) write privileges.

The return code will indicate success or failure.
