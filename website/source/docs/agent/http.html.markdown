---
layout: "docs"
page_title: "HTTP API"
sidebar_current: "docs-agent-http"
description: |-
  The main interface to Consul is a RESTful HTTP API. The API can be used for CRUD for nodes, services, checks, and configuration. The endpoints are versioned to enable changes without breaking backwards compatibility.
---

# HTTP API

The main interface to Consul is a RESTful HTTP API. The API can be
used for CRUD for nodes, services, checks, and configuration. The endpoints are
versioned to enable changes without breaking backwards compatibility.

All endpoints fall into one of several categories:

* [kv][kv] - Key/Value store
* [agent][agent] - Agent control
* [catalog][catalog] - Manages nodes and services
* [health][health] - Manages health checks
* [session][session] - Session manipulation
* [acl][acl] - ACL creations and management
* [event][event] - User Events
* [status][status] - Consul system status
* internal - Internal APIs. Purposely undocumented, subject to change.

Each of the categories and their respective endpoints are documented below.

## Blocking Queries

Certain endpoints support a feature called a "blocking query." A blocking query
is used to wait for a change to potentially take place using long polling.

Queries that support this will mention it specifically, however the use of this
feature is the same for all. If supported, the query will set an HTTP header
"X-Consul-Index". This is an opaque handle that the client will use.

To cause a query to block, the query parameters "?wait=\<interval\>&index=\<idx\>" are added
to a request. The "?wait=" query parameter limits how long the query will potentially
block for. It not set, it will default to 10 minutes. It can be specified in the form of
"10s" or "5m", which is 10 seconds or 5 minutes respectively. The "?index=" parameter is an
opaque handle, which is used by Consul to detect changes. The  "X-Consul-Index" header for a
query provides this value, and can be used to wait for changes since the query was run.

When provided, Consul blocks sending a response until there is an update that
could have cause the output to change, and thus advancing the index. A critical
note is that when the query returns there is **no guarantee** of a change. It is
possible that the timeout was reached, or that there was an idempotent write that
does not affect the result.

## Consistency Modes

Most of the read query endpoints support multiple levels of consistency.
These are to provide a tuning knob that clients can be used to find a happy
medium that best matches their needs.

The three read modes are:

* default - If not specified, this mode is used. It is strongly consistent
  in almost all cases. However, there is a small window in which an new
  leader may be elected, and the old leader may service stale values. The
  trade off is fast reads, but potentially stale values. This condition is
  hard to trigger, and most clients should not need to worry about the stale read.
  This only applies to reads, and a split-brain is not possible on writes.

* consistent - This mode is strongly consistent without caveats. It requires
  that a leader verify with a quorum of peers that it is still leader. This
  introduces an additional round-trip to all server nodes. The trade off is
  always consistent reads, but increased latency due to an extra round trip.
  Most clients should not use this unless they cannot tolerate a stale read.

* stale - This mode allows any server to service the read, regardless of if
  it is the leader. This means reads can be arbitrarily stale, but are generally
  within 50 milliseconds of the leader. The trade off is very fast and scalable
  reads but values will be stale. This mode allows reads without a leader, meaning
  a cluster that is unavailable will still be able to respond.

To switch these modes, either the "?stale" or "?consistent" query parameters
are provided. It is an error to provide both.

To support bounding how stale data is, there is an "X-Consul-LastContact"
which is the last time a server was contacted by the leader node in
milliseconds. The "X-Consul-KnownLeader" also indicates if there is a known
leader. These can be used to gauge if a stale read should be used.

## Formatted JSON Output

By default, the output of all HTTP API requests return minimized JSON with all
whitespace removed.  By adding "?pretty=1" to the HTTP request URL,
formatted JSON will be returned.

## ACLs

Several endpoints in Consul use or require ACL tokens to operate. An agent
can be configured to use a default token in requests using the `acl_token`
configuration option. However, the token can also be specified per-request
by using the "?token=" query parameter. This will take precedence over the
default token.

## <a name="kv"></a> KV

The KV endpoint is used to expose a simple key/value store. This can be used
to store service configurations or other meta data in a simple way. It has only
a single endpoint:

    /v1/kv/<key>

This is the only endpoint that is used with the Key/Value store.
Its use depends on the HTTP method. The `GET`, `PUT` and `DELETE` methods
are all supported. It is important to note that each datacenter has its
own K/V store, and that there is no replication between datacenters.
By default the datacenter of the agent is queried, however the dc can
be provided using the "?dc=" query parameter. If a client wants to write
to all Datacenters, one request per datacenter must be made. The KV endpoint
supports the use of ACL tokens.

### GET Method

When using the `GET` method, Consul will return the specified key,
or if the "?recurse" query parameter is provided, it will return
all keys with the given prefix.

Each object will look like:

```javascript
[
  {
    "CreateIndex": 100,
    "ModifyIndex": 200,
    "LockIndex": 200,
    "Key": "zip",
    "Flags": 0,
    "Value": "dGVzdA==",
    "Session": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
  }
]
```

The `CreateIndex` is the internal index value that represents
when the entry was created. The `ModifyIndex` is the last index
that modified this key. This index corresponds to the `X-Consul-Index`
header value that is returned. A blocking query can be used to wait for
a value to change. If "?recurse" is used, the `X-Consul-Index` corresponds
to the latest `ModifyIndex` and so a blocking query waits until any of the
listed keys are updated.  The `LockIndex` is the last index of a successful
lock acquisition. If the lock is held, the `Session` key provides the
session that owns the lock.

The `Key` is simply the full path of the entry. `Flags` are an opaque
unsigned integer that can be attached to each entry. The use of this is
left totally to the user. Lastly, the `Value` is a base64 key value.

It is possible to also only list keys without their values by using the
"?keys" query parameter along with a `GET` request. This will return
a list of the keys under the given prefix. The optional "?separator="
can be used to list only up to a given separator.

For example, listing "/web/" with a "/" separator may return:

```javascript
[
  "/web/bar",
  "/web/foo",
  "/web/subdir/"
]
```

Using the key listing method may be suitable when you do not need
the values or flags, or want to implement a key-space explorer.

If the "?raw" query parameter is used with a non-recursive GET,
then the response is just the raw value of the key, without any
encoding.

If no entries are found, a 404 code is returned.

This endpoint supports blocking queries and all consistency modes.

### PUT method

When using the `PUT` method, Consul expects the request body to be the
value corresponding to the key. There are a number of parameters that can
be used with a PUT request:

* ?flags=\<num\> : This can be used to specify an unsigned value between
  0 and 2^64-1. It is opaque to the user, but a client application may
  use it.

* ?cas=\<index\> : This flag is used to turn the `PUT` into a Check-And-Set
  operation. This is very useful as it allows clients to build more complex
  synchronization primitives on top. If the index is 0, then Consul will only
  put the key if it does not already exist. If the index is non-zero, then
  the key is only set if the index matches the `ModifyIndex` of that key.

* ?acquire=\<session\> : This flag is used to turn the `PUT` into a lock acquisition
  operation. This is useful as it allows leader election to be built on top
  of Consul. If the lock is not held and the session is valid, this increments
  the `LockIndex` and sets the `Session` value of the key in addition to updating
  the key contents. A key does not need to exist to be acquired.

* ?release=\<session\> : This flag is used to turn the `PUT` into a lock release
  operation. This is useful when paired with "?acquire=" as it allows clients to
  yield a lock. This will leave the `LockIndex` unmodified but will clear the associated
  `Session` of the key. The key must be held by this session to be unlocked.

The return value is simply either `true` or `false`. If `false` is returned,
then the update has not taken place.

### DELETE method

Lastly, the `DELETE` method can be used to delete a single key or all
keys sharing a prefix. If the "?recurse" query parameter is provided,
then all keys with the prefix are deleted, otherwise only the specified
key.

## <a name="agent"></a> Agent

The Agent endpoints are used to interact with a local Consul agent. Usually,
services and checks are registered with an agent, which then takes on the
burden of registering with the Catalog and performing anti-entropy to recover from
outages. There are also various control APIs that can be used instead of the
msgpack RPC protocol.

The following endpoints are supported:

* [`/v1/agent/checks`](#agent_checks) : Returns the checks the local agent is managing
* [`/v1/agent/services`](#agent_services) : Returns the services local agent is managing
* [`/v1/agent/members`](#agent_members) : Returns the members as seen by the local serf agent
* [`/v1/agent/self`](#agent_self) : Returns the local node configuration
* [`/v1/agent/join/<address>`](#agent_join) : Trigger local agent to join a node
* [`/v1/agent/force-leave/<node>`](#agent_force_leave)>: Force remove node
* [`/v1/agent/check/register`](#agent_check_register) : Registers a new local check
* [`/v1/agent/check/deregister/<checkID>`](#agent_check_deregister) : Deregister a local check
* [`/v1/agent/check/pass/<checkID>`](#agent_check_pass) : Mark a local test as passing
* [`/v1/agent/check/warn/<checkID>`](#agent_check_warn) : Mark a local test as warning
* [`/v1/agent/check/fail/<checkID>`](#agent_check_fail) : Mark a local test as critical
* [`/v1/agent/service/register`](#agent_service_register) : Registers a new local service
* [`/v1/agent/service/deregister/<serviceID>`](#agent_service_deregister) : Deregister a local service

### <a name="agent_checks"></a> /v1/agent/checks

This endpoint is used to return the all the checks that are registered with
the local agent. These checks were either provided through configuration files,
or added dynamically using the HTTP API. It is important to note that the checks
known by the agent may be different than those reported by the Catalog. This is usually
due to changes being made while there is no leader elected. The agent performs active
anti-entropy, so in most situations everything will be in sync within a few seconds.

This endpoint is hit with a GET and returns a JSON body like this:

```javascript
{
  "service:redis": {
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis"
  }
}
```

### <a name="agent_services"></a> /v1/agent/services

This endpoint is used to return the all the services that are registered with
the local agent. These services were either provided through configuration files,
or added dynamically using the HTTP API. It is important to note that the services
known by the agent may be different than those reported by the Catalog. This is usually
due to changes being made while there is no leader elected. The agent performs active
anti-entropy, so in most situations everything will be in sync within a few seconds.

This endpoint is hit with a GET and returns a JSON body like this:

```javascript
{
  "redis": {
    "ID": "redis",
    "Service": "redis",
    "Tags": null,
    "Port": 8000
  }
}
```

### <a name="agent_members"></a> /v1/agent/members

This endpoint is hit with a GET and returns the members the agent sees in the
cluster gossip pool. Due to the nature of gossip, this is eventually consistent
and the results may differ by agent. The strongly consistent view of nodes is
instead provided by "/v1/catalog/nodes".

For agents running in server mode, providing a "?wan=1" query parameter returns
the list of WAN members instead of the LAN members which is default.

This endpoint returns a JSON body like:

```javascript
[
  {
    "Name": "foobar",
    "Addr": "10.1.10.12",
    "Port": 8301,
    "Tags": {
      "bootstrap": "1",
      "dc": "dc1",
      "port": "8300",
      "role": "consul"
    },
    "Status": 1,
    "ProtocolMin": 1,
    "ProtocolMax": 2,
    "ProtocolCur": 2,
    "DelegateMin": 1,
    "DelegateMax": 3,
    "DelegateCur": 3
  }
]
```

### <a name="agent_self"></a> /v1/agent/self

This endpoint is used to return configuration of the local agent and member information.

It returns a JSON body like this:

```javascript
{
  "Config": {
    "Bootstrap": true,
    "Server": true,
    "Datacenter": "dc1",
    "DataDir": "/tmp/consul",
    "DNSRecursor": "",
    "DNSRecursors": [],
    "Domain": "consul.",
    "LogLevel": "INFO",
    "NodeName": "foobar",
    "ClientAddr": "127.0.0.1",
    "BindAddr": "0.0.0.0",
    "AdvertiseAddr": "10.1.10.12",
    "Ports": {
      "DNS": 8600,
      "HTTP": 8500,
      "RPC": 8400,
      "SerfLan": 8301,
      "SerfWan": 8302,
      "Server": 8300
    },
    "LeaveOnTerm": false,
    "SkipLeaveOnInt": false,
    "StatsiteAddr": "",
    "Protocol": 1,
    "EnableDebug": false,
    "VerifyIncoming": false,
    "VerifyOutgoing": false,
    "CAFile": "",
    "CertFile": "",
    "KeyFile": "",
    "StartJoin": [],
    "UiDir": "",
    "PidFile": "",
    "EnableSyslog": false,
    "RejoinAfterLeave": false
  },
  "Member": {
    "Name": "foobar",
    "Addr": "10.1.10.12",
    "Port": 8301,
    "Tags": {
      "bootstrap": "1",
      "dc": "dc1",
      "port": "8300",
      "role": "consul",
      "vsn": "1",
      "vsn_max": "1",
      "vsn_min": "1"
    },
    "Status": 1,
    "ProtocolMin": 1,
    "ProtocolMax": 2,
    "ProtocolCur": 2,
    "DelegateMin": 2,
    "DelegateMax": 4,
    "DelegateCur": 4
  }
}
```

### <a name="agent_join"></a> /v1/agent/join/\<address\>

This endpoint is hit with a GET and is used to instruct the agent to attempt to
connect to a given address.  For agents running in server mode, providing a "?wan=1"
query parameter causes the agent to attempt to join using the WAN pool.

The endpoint returns 200 on successful join.

### <a name="agent_force_leave"></a> /v1/agent/force-leave/\<node\>

This endpoint is hit with a GET and is used to instructs the agent to force a node into the left state.
If a node fails unexpectedly, then it will be in a "failed" state. Once in this state, Consul will
attempt to reconnect, and additionally the services and checks belonging to that node will not be
cleaned up. Forcing a node into the left state allows its old entries to be removed.

The endpoint always returns 200.

### <a name="agent_check_register"></a> /v1/agent/check/register

The register endpoint is used to add a new check to the local agent.
There is more documentation on checks [here](/docs/agent/checks.html).
Checks are either a script or TTL type. The agent is responsible for managing
the status of the check and keeping the Catalog in sync.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "ID": "mem",
  "Name": "Memory utilization",
  "Notes": "Ensure we don't oversubscribe memory",
  "Script": "/usr/local/bin/check_mem.py",
  "Interval": "10s",
  "TTL": "15s"
}
```

The `Name` field is mandatory, as is either `Script` and `Interval`
or `TTL`. Only one of `Script` and `Interval` or `TTL` should be provided.
If an `ID` is not provided, it is set to `Name`. You cannot have duplicate
`ID` entries per agent, so it may be necessary to provide an ID. The `Notes`
field is not used by Consul, and is meant to be human readable.

If a `Script` is provided, the check type is a script, and Consul will
evaluate the script every `Interval` to update the status. If a `TTL` type
is used, then the TTL update APIs must be used to periodically update
the state of the check.

The return code is 200 on success.

### <a name="agent_check_deregister"></a> /v1/agent/check/deregister/\<checkId\>

The deregister endpoint is used to remove a check from the local agent.
The CheckID must be passed after the slash. The agent will take care
of deregistering the check with the Catalog.

The return code is 200 on success.

### <a name="agent_check_pass"></a> /v1/agent/check/pass/\<checkId\>

This endpoint is used with a check that is of the [TTL type](/docs/agent/checks.html).
When this endpoint is accessed via a GET, the status of the check is set to "passing",
and the TTL clock is reset.

The optional "?note=" query parameter can be used to associate output with
the status of the check. This should be human readable for operators.

The return code is 200 on success.

### <a name="agent_check_warn"></a> /v1/agent/check/warn/\<checkId\>

This endpoint is used with a check that is of the [TTL type](/docs/agent/checks.html).
When this endpoint is accessed via a GET, the status of the check is set to "warning",
and the TTL clock is reset.

The optional "?note=" query parameter can be used to associate output with
the status of the check. This should be human readable for operators.

The return code is 200 on success.

### <a name="agent_check_fail"></a> /v1/agent/check/fail/\<checkId\>

This endpoint is used with a check that is of the [TTL type](/docs/agent/checks.html).
When this endpoint is accessed via a GET, the status of the check is set to "critical",
and the TTL clock is reset.

The optional "?note=" query parameter can be used to associate output with
the status of the check. This should be human readable for operators.

The return code is 200 on success.

### <a name="agent_service_register"></a> /v1/agent/service/register

The register endpoint is used to add a new service to the local agent.
There is more documentation on services [here](/docs/agent/services.html).
Services may also provide a health check. The agent is responsible for managing
the status of the check and keeping the Catalog in sync.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "ID": "redis1",
  "Name": "redis",
  "Tags": [
    "master",
    "v1"
  ],
  "Port": 8000,
  "Check": {
    "Script": "/usr/local/bin/check_redis.py",
    "Interval": "10s",
    "TTL": "15s"
  }
}
```

The `Name` field is mandatory,  If an `ID` is not provided, it is set to `Name`.
You cannot have duplicate `ID` entries per agent, so it may be necessary to provide an ID.
`Tags`, `Port` and `Check` are optional. If `Check` is provided, only one of `Script` and `Interval`
or `TTL` should be provided. There is more information about checks [here](/docs/agent/checks.html).

The created check will be named "service:\<ServiceId\>".

The return code is 200 on success.

### <a name="agent_service_deregister"></a> /v1/agent/service/deregister/\<serviceId\>

The deregister endpoint is used to remove a service from the local agent.
The ServiceID must be passed after the slash. The agent will take care
of deregistering the service with the Catalog. If there is an associated
check, that is also deregistered.

The return code is 200 on success.

## <a name="catalog"></a> Catalog

The Catalog is the endpoint used to register and deregister nodes,
services, and checks. It also provides a number of query endpoints.

The following endpoints are supported:

* [`/v1/catalog/register`](#catalog_register) : Registers a new node, service, or check
* [`/v1/catalog/deregister`](#catalog_deregister) : Deregisters a node, service, or check
* [`/v1/catalog/datacenters`](#catalog_datacenters) : Lists known datacenters
* [`/v1/catalog/nodes`](#catalog_nodes) : Lists nodes in a given DC
* [`/v1/catalog/services`](#catalog_services) : Lists services in a given DC
* [`/v1/catalog/service/<service>`](#catalog_service) : Lists the nodes in a given service
* [`/v1/catalog/node/<node>`](#catalog_nodes) : Lists the services provided by a node

The last 4 endpoints of the catalog support blocking queries and
consistency modes.

### <a name="catalog_register"></a> /v1/catalog/register

The register endpoint is a low level mechanism for directly registering
or updating entries in the catalog. It is usually recommended to use
the agent local endpoints, as they are simpler and perform anti-entropy.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "Address": "192.168.10.10",
  "Service": {
    "ID": "redis1",
    "Service": "redis",
    "Tags": [
      "master",
      "v1"
    ],
    "Port": 8000
  },
  "Check": {
    "Node": "foobar",
    "CheckID": "service:redis1",
    "Name": "Redis health check",
    "Notes": "Script based health check",
    "Status": "passing",
    "ServiceID": "redis1"
  }
}
```

The behavior of the endpoint depends on what keys are provided. The endpoint
requires `Node` and `Address` to be provided, while `Datacenter` will be defaulted
to match that of the agent. If only those are provided, the endpoint will register
the node with the catalog.

If the `Service` key is provided, then the service will also be registered. If
`ID` is not provided, it will be defaulted to `Service`. It is mandated that the
ID be node-unique. Both `Tags` and `Port` can be omitted.

If the `Check` key is provided, then a health check will also be registered. It
is important to remember that this register API is very low level. This manipulates
the health check entry, but does not setup a script or TTL to actually update the
status. For that behavior, an agent local check should be setup.

The `CheckID` can be omitted, and will default to the `Name`. Like before, the
`CheckID` must be node-unique. The `Notes` is an opaque field that is meant to
hold human readable text. If a `ServiceID` is provided that matches the `ID`
of a service on that node, then the check is treated as a service level health
check, instead of a node level health check. Lastly, the status must be one of
"unknown", "passing", "warning", or "critical". The "unknown" status is used
to indicate that the initial check has not been performed yet.

It is important to note that `Check` does not have to be provided with `Service`
and visa-versa. They can be provided or omitted at will.

If the API call succeeds a 200 status code is returned.

### <a name="catalog_deregister"></a> /v1/catalog/deregister

The deregister endpoint is a low level mechanism for directly removing
entries in the catalog. It is usually recommended to use the agent local
endpoints, as they are simpler and perform anti-entropy.

The deregister endpoint expects a JSON request body to be PUT. The request
body must look like one of the following:

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
}
```

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "CheckID": "service:redis1"
}
```

```javascript
{
  "Datacenter": "dc1",
  "Node": "foobar",
  "ServiceID": "redis1",
}
```

The behavior of the endpoint depends on what keys are provided. The endpoint
requires `Node` to be provided, while `Datacenter` will be defaulted
to match that of the agent. If only `Node` is provided, then the node, and
all associated services and checks are deleted. If `CheckID` is provided, only
that check belonging to the node is removed. If `ServiceID` is provided, then the
service along with its associated health check (if any) is removed.

If the API call succeeds a 200 status code is returned.


### <a name="catalog_datacenters"></a> /v1/catalog/datacenters

This endpoint is hit with a GET and is used to return all the
datacenters that are known by the Consul server.

It returns a JSON body like this:

```javascript
["dc1", "dc2"]
```

This endpoint does not require a cluster leader, and as such
will succeed even during an availability outage. It can thus be
a simple check to see if any Consul servers are routable.

### <a name="catalog_nodes"></a> /v1/catalog/nodes

This endpoint is hit with a GET and returns the nodes known
about in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "baz",
    "Address": "10.1.10.11"
  },
  {
    "Node": "foobar",
    "Address": "10.1.10.12"
  }
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_services"></a> /v1/catalog/services

This endpoint is hit with a GET and returns the services known
about in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

```javascript
{
  "consul": [],
  "redis": [],
  "postgresql": [
    "master",
    "slave"
  ]
}
```

The main object keys are the service names, while the array
provides all the known tags for a given service.

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_service"></a> /v1/catalog/service/\<service\>

This endpoint is hit with a GET and returns the nodes providing a service
in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

The service being queried must be provided after the slash. By default
all nodes in that service are returned. However, the list can be filtered
by tag using the "?tag=" query parameter.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "foobar",
    "Address": "10.1.10.12",
    "ServiceID": "redis",
    "ServiceName": "redis",
    "ServiceTags": null,
    "ServicePort": 8000
  }
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="catalog_node"></a> /v1/catalog/node/\<node\>

This endpoint is hit with a GET and returns the node provided services.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The node being queried must be provided after the slash.

It returns a JSON body like this:

```javascript
{
  "Node": {
    "Node": "foobar",
    "Address": "10.1.10.12"
  },
  "Services": {
    "consul": {
      "ID": "consul",
      "Service": "consul",
      "Tags": null,
      "Port": 8300
    },
    "redis": {
      "ID": "redis",
      "Service": "redis",
      "Tags": [
        "v1"
      ],
      "Port": 8000
    }
  }
}
```

This endpoint supports blocking queries and all consistency modes.

## <a name="health"></a> Health

The Health used to query health related information. It is provided separately
from the Catalog, since users may prefer to not use the health checking mechanisms
as they are totally optional. Additionally, some of the query results from the Health system are filtered, while the Catalog endpoints provide the raw entries.

The following endpoints are supported:

* [`/v1/health/node/<node>`](#health_node): Returns the health info of a node
* [`/v1/health/checks/<service>`](#health_checks): Returns the checks of a service
* [`/v1/health/service/<service>`](#health_service): Returns the nodes and health info of a service
* [`/v1/health/state/<state>`](#health_state): Returns the checks in a given state

All of the health endpoints supports blocking queries and all consistency modes.

### <a name="health_node"></a> /v1/health/node/\<node\>

This endpoint is hit with a GET and returns the node specific checks known.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The node being queried must be provided after the slash.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "foobar",
    "CheckID": "serfHealth",
    "Name": "Serf Health Status",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "",
    "ServiceName": ""
  },
  {
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis"
  }
]
```

In this case, we can see there is a system level check (no associated
`ServiceID`, as well as a service check for Redis). The "serfHealth" check
is special, in that all nodes automatically have this check. When a node
joins the Consul cluster, it is part of a distributed failure detection
provided by Serf. If a node fails, it is detected and the status is automatically
changed to "critical".

This endpoint supports blocking queries and all consistency modes.

### <a name="health_checks"></a> /v1/health/checks/\<service\>

This endpoint is hit with a GET and returns the checks associated with
a service in a given datacenter.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The service being queried must be provided after the slash.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis"
  }
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="health_service"></a> /v1/health/service/\<service\>

This endpoint is hit with a GET and returns the service nodes providing
a given service in a given datacenter.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

The service being queried must be provided after the slash. By default
all nodes in that service are returned. However, the list can be filtered
by tag using the "?tag=" query parameter.

This is very similar to the /v1/catalog/service endpoint however, this
endpoint automatically returns the status of the associated health check,
as well as any system level health checks. This allows a client to avoid
sending traffic to nodes failing health tests, or who are reporting warnings.

Providing the "?passing" query parameter will filter results to only nodes
with all checks in the passing state. This can be used to avoid some filtering
logic on the client side. (Added in Consul 0.2)

Users can also built in support for dynamic load balancing and other features
by incorporating the use of health checks.

It returns a JSON body like this:

```javascript
[
  {
    "Node": {
      "Node": "foobar",
      "Address": "10.1.10.12"
    },
    "Service": {
      "ID": "redis",
      "Service": "redis",
      "Tags": null,
      "Port": 8000
    },
    "Checks": [
      {
        "Node": "foobar",
        "CheckID": "service:redis",
        "Name": "Service 'redis' check",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "redis",
        "ServiceName": "redis"
      },
      {
        "Node": "foobar",
        "CheckID": "serfHealth",
        "Name": "Serf Health Status",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "",
        "ServiceName": ""
      }
    ]
  }
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="health_state"></a> /v1/health/state/\<state\>

This endpoint is hit with a GET and returns the checks in a specific
state for a given datacenter. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

The state being queried must be provided after the slash. The supported states
are "any", "unknown", "passing", "warning", or "critical". The "any" state is
a wildcard that can be used to return all the checks.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "foobar",
    "CheckID": "serfHealth",
    "Name": "Serf Health Status",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "",
    "ServiceName": ""
  },
  {
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis"
  }
]
```

This endpoint supports blocking queries and all consistency modes.

## <a name="session"></a> Session

The Session endpoints are used to create, destroy and query sessions.
The following endpoints are supported:

* [`/v1/session/create`](#session_create): Creates a new session
* [`/v1/session/destroy/<session>`](#session_destroy): Destroys a given session
* [`/v1/session/info/<session>`](#session_info): Queries a given session
* [`/v1/session/node/<node>`](#session_node): Lists sessions belonging to a node
* [`/v1/session/list`](#session_list): Lists all the active sessions

All of the read session endpoints supports blocking queries and all consistency modes.

### <a name="session_create"></a> /v1/session/create

The create endpoint is used to initialize a new session.
There is more documentation on sessions [here](/docs/internals/sessions.html).
Sessions must be associated with a node, and optionally any number of checks.
By default, the agent uses it's own node name, and provides the "serfHealth"
check, along with a 15 second lock delay.

By default, the agent's local datacenter is used, but another datacenter
can be specified using the "?dc=" query parameter. It is not recommended
to use cross-region sessions.

The create endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "LockDelay": "15s",
  "Name": "my-service-lock",
  "Node": "foobar",
  "Checks": ["a", "b", "c"]
}
```

None of the fields are mandatory, and in fact no body needs to be PUT
if the defaults are to be used. The `LockDelay` field can be specified
as a duration string using a "s" suffix for seconds. It can also be a numeric
value. Small values are treated as seconds, and otherwise it is provided with
nanosecond granularity.

The `Node` field must refer to a node that is already registered. By default,
the agent will use it's own name. The `Name` field can be used to provide a human
readable name for the Session. Lastly, the `Checks` field is used to provide
a list of associated health checks. By default the "serfHealth" check is provided.
It is highly recommended that if you override this list, you include that check.

The return code is 200 on success, along with a body like:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

This is used to provide the ID of the newly created session.

### <a name="session_destroy"></a> /v1/session/destroy/\<session\>

The destroy endpoint is hit with a PUT and destroys the given session.
By default the local datacenter is used, but the "?dc=" query parameter
can be used to specify the datacenter. The session being destroyed must
be provided after the slash.

The return code is 200 on success.

### <a name="session_info"></a> /v1/session/info/\<session\>

This endpoint is hit with a GET and returns the session information
by ID within a given datacenter. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The session being queried must be provided after the slash.

It returns a JSON body like this:

```javascript
[
  {
    "LockDelay": 1.5e+10,
    "Checks": [
      "serfHealth"
    ],
    "Node": "foobar",
    "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "CreateIndex": 1086449
  }
]
```

If the session is not found, null is returned instead of a JSON list.
This endpoint supports blocking queries and all consistency modes.

### <a name="session_node"></a> /v1/session/node/\<node\>

This endpoint is hit with a GET and returns the active sessions
for a given node and datacenter. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The node being queried must be provided after the slash.

It returns a JSON body like this:

```javascript
[
  {
    "LockDelay": 1.5e+10,
    "Checks": [
      "serfHealth"
    ],
    "Node": "foobar",
    "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "CreateIndex": 1086449
  },
  ...
]
```

This endpoint supports blocking queries and all consistency modes.

### <a name="session_list"></a> /v1/session/list

This endpoint is hit with a GET and returns the active sessions
for a given datacenter. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

```javascript
[
  {
    "LockDelay": 1.5e+10,
    "Checks": [
      "serfHealth"
    ],
    "Node": "foobar",
    "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "CreateIndex": 1086449
  },
  ...
]
```

This endpoint supports blocking queries and all consistency modes.

## <a name="acl"></a> ACL

The ACL endpoints are used to create, update, destroy and query ACL tokens.
The following endpoints are supported:

* [`/v1/acl/create`](#acl_create): Creates a new token with policy
* [`/v1/acl/update`](#acl_update): Update the policy of a token
* [`/v1/acl/destroy/<id>`](#acl_destroy): Destroys a given token
* [`/v1/acl/info/<id>`](#acl_info): Queries the policy of a given token
* [`/v1/acl/clone/<id>`](#acl_clone): Creates a new token by cloning an existing token
* [`/v1/acl/list`](#acl_list): Lists all the active tokens

### <a name="acl_create"></a> /v1/acl/create

The create endpoint is used to make a new token. A token has a name,
type, and a set of ACL rules. The name is opaque to Consul, and type
is either "client" or "management". A management token is effectively
like a root user, and has the ability to perform any action including
creating, modifying, and deleting ACLs. A client token can only perform
actions as permitted by the rules associated, and may never manage ACLs.
This means the request to this endpoint must be made with a management
token.

In any Consul cluster, only a single datacenter is authoritative for ACLs, so
all requests are automatically routed to that datacenter regardless
of the agent that the request is made to.

The create endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "Name": "my-app-token",
  "Type": "client",
  "Rules": ""
}
```

None of the fields are mandatory, and in fact no body needs to be PUT
if the defaults are to be used. The `Name` and `Rules` default to being
blank, and the `Type` defaults to "client". The format of `Rules` is
[documented here](/docs/internals/acl.html).

The return code is 200 on success, along with a body like:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

This is used to provide the ID of the newly created ACL token.

### <a name="acl_update"></a> /v1/acl/update

The update endpoint is used to modify the policy for a given
ACL token. It is very similar to the create endpoint, however
instead of generating a new token ID, the `ID` field must be
provided. Requests to this endpoint must be made with a management
token.

In any Consul cluster, only a single datacenter is authoritative for ACLs, so
all requests are automatically routed to that datacenter regardless
of the agent that the request is made to.

The update endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
  "Name": "my-app-token-updated",
  "Type": "client",
  "Rules": "# New Rules",
}
```

Only the `ID` field is mandatory, the other fields provide defaults.
The `Name` and `Rules` default to being blank, and the `Type` defaults to "client".
The format of `Rules` is [documented here](/docs/internals/acl.html).

The return code is 200 on success.

### <a name="acl_destroy"></a> /v1/acl/destroy/\<id\>

The destroy endpoint is hit with a PUT and destroys the given ACL token.
The request is automatically routed to the authoritative ACL datacenter.
The token being destroyed must be provided after the slash, and requests
to the endpoint must be made with a management token.

The return code is 200 on success.

### <a name="acl_info"></a> /v1/acl/info/\<id\>

This endpoint is hit with a GET and returns the token information
by ID. All requests are routed to the authoritative ACL datacenter
The token being queried must be provided after the slash.

It returns a JSON body like this:

```javascript
[
  {
    "CreateIndex": 3,
    "ModifyIndex": 3,
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "Name": "Client Token",
    "Type": "client",
    "Rules": "..."
  }
]
```

If the session is not found, null is returned instead of a JSON list.

### <a name="acl_clone"></a> /v1/acl/clone/\<id\>

The clone endpoint is hit with a PUT and returns a token ID that
is cloned from an existing token. This allows a token to serve
as a template for others, making it simple to generate new tokens
without complex rule management. The source token must be provided
after the slash. Requests to this endpoint require a management token.

The return code is 200 on success, along with a body like:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

This is used to provide the ID of the newly created ACL token.

### <a name="acl_list"></a> /v1/acl/list

The list endpoint is hit with a GET and lists all the active
ACL tokens. This is a privileged endpoint, and requires a
management token.

It returns a JSON body like this:

```javascript
[
  {
    "CreateIndex": 3,
    "ModifyIndex": 3,
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "Name": "Client Token",
    "Type": "client",
    "Rules": "..."
  },
  ...
]
```

## <a name="event"></a> Event

The Event endpoints are used to fire new events and to query the available
events.

The following endpoints are supported:

* [`/v1/event/fire/<name>`](#event_fire): Fires a new user event
* [`/v1/event/list`](#event_list): Lists the most recent events an agent has seen.

### <a name="event_fire"></a> /v1/event/fire/\<name\>

The fire endpoint is used to trigger a new user event. A user event
needs a name, and optionally takes a number of parameters.

By default, the agent's local datacenter is used, but another datacenter
can be specified using the "?dc=" query parameter.

The fire endpoint expects a PUT request, with an optional body.
The body contents are opaque to Consul, and become the "payload"
of the event. Any names starting with the "_" prefix should be considered
reserved, and for Consul's internal use.

The `?node=`, `?service=`, and `?tag=` query parameters may optionally
be provided. They respectively provide a regular expression to filter
by node name, service, and service tags.

The return code is 200 on success, along with a body like:

```javascript
{
  "ID": "b54fe110-7af5-cafc-d1fb-afc8ba432b1c",
  "Name": "deploy",
  "Payload": null,
  "NodeFilter": "",
  "ServiceFilter": "",
  "TagFilter": "",
  "Version": 1,
  "LTime": 0
}
```

This is used to provide the ID of the newly fired event.

### <a name="event_list"></a> /v1/event/list

This endpoint is hit with a GET and returns the most recent
events known by the agent. As a consequence of how the
[event command](/docs/commands/event.html) works, each agent
may have a different view of the events. Events are broadcast using
the [gossip protocol](/docs/internals/gossip.html), which means
they have no total ordering, nor do they make a promise of delivery.

Additionally, each node applies the node, service and tag filters
locally before storing the event. This means the events at each agent
may be different depending on their configuration.

This endpoint does allow for filtering on events by name by providing
the `?name=` query parameter.

Lastly, to support [watches](/docs/agent/watches.html), this endpoint
supports blocking queries. However, the semantics of this endpoint
are slightly different. Most blocking queries provide a monotonic index,
and block until a newer index is available. This can be supported as
a consequence of the total ordering of the [consensus protocol](/docs/internals/consensus.html).
With gossip, there is no ordering, and instead `X-Consul-Index` maps
to the newest event that matches the query.

In practice, this means the index is only useful when used against a
single agent, and has no meaning globally. Because Consul defines
the index as being opaque, clients should not be expecting a natural
ordering either.

Lastly, agents only buffer the most recent entries. The number
of entries should not be depended upon, but currently defaults to
256. This value could change in the future. The buffer should be large
enough for most clients and watches.

It returns a JSON body like this:

```javascript
[
  {
    "ID": "b54fe110-7af5-cafc-d1fb-afc8ba432b1c",
    "Name": "deploy",
    "Payload": "MTYwOTAzMA==",
    "NodeFilter": "",
    "ServiceFilter": "",
    "TagFilter": "",
    "Version": 1,
    "LTime": 19
  },
  ...
]
```

## <a name="status"></a> Status

The Status endpoints are used to get information about the status
of the Consul cluster. These are generally very low level, and not really
useful for clients.

The following endpoints are supported:

* [`/v1/status/leader`](#status_leader) : Returns the current Raft leader
* [`/v1/status/peers`](#status_peers) : Returns the current Raft peer set

### <a name="status_leader"></a> /v1/status/leader

This endpoint is used to get the Raft leader for the datacenter
the agent is running in. It returns only an address like:

```text
"10.1.10.12:8300"
```

### <a name="status_peers"></a> /v1/status/peers

This endpoint is used to get the Raft peers for the datacenter
the agent is running in. It returns a list of addresses like:

```javascript
[
  "10.1.10.12:8300",
  "10.1.10.11:8300",
  "10.1.10.10:8300"
]
```

[kv]: #kv
[agent]: #agent
[catalog]: #catalog
[health]: #health
[session]: #session
[acl]: #acl
[event]: #event
[status]: #status
