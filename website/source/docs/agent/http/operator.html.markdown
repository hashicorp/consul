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

The following endpoints are supported:

* [`/v1/operator/raft/configuration`](#raft-configuration): Inspects the Raft configuration
* [`/v1/operator/raft/peer`](#raft-peer): Operates on Raft peers
* [`/v1/operator/keyring`](#keyring): Operates on gossip keyring

Not all endpoints support blocking queries and all consistency modes,
see details in the sections below.

The operator endpoints support the use of ACL Tokens. See the
[ACL](/docs/internals/acl.html#operator) internals guide for more information.

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
but may  be upgraded to a GUID in a future version of Consul.

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

### <a name="keyring"></a> /v1/operator/keyring

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
