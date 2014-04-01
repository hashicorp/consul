---
layout: "docs"
page_title: "HTTP API"
sidebar_current: "docs-agent-http"
---

# HTTP API

The main interface to Consul is a RESTful HTTP API. The API can be
used for CRUD for nodes, services, checks, and configuration. The endpoints are
versioned to enable changes without breaking backwards compatibility.

All endpoints fall into one of 5 categories:

* kv - Key/Value store
* agent - Agent control
* catalog - Manages nodes and services
* health - Manages health checks
* status - Consul system status

Each of the categories and their respective endpoints are documented below.

## Blocking Queries

Certain endpoints support a feature called a "blocking query". A blocking query
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


## KV

The KV endpoint is used to expose a simple key/value store. This can be used
to store service configurations or other meta data in a simple way. It has only
a single endpoint:

    /v1/kv/<key>

This is the only endpoint that is used with the Key/Value store.
It's use depends on the HTTP method. The `GET`, `PUT` and `DELETE` methods
are all supported.

When using the `GET` method, Consul will return the specified key,
or if the "?recurse" query parameter is provided, it will return
all keys with the given prefix.

Each object will look like:

    [
        {
            "CreateIndex":100,
            "ModifyIndex":200,
            "Key":"zip",
            "Flags":0,
            "Value":"dGVzdA=="
        }
    ]

The `CreateIndex` is the internal index value that represents
when the entry was created. The `ModifyIndex` is the last index
that modified this key. This index corresponds to the `X-Consul-Index`
header value that is returned. A blocking query can be used to wait for
a value to change. If "?recurse" is used, the `X-Consul-Index` corresponds
to the latest `ModifyIndex` and so a blocking query waits until any of the
listed keys are updated.

The `Key` is simply the full path of the entry. `Flags` are an opaque
unsigned integer that can be attached to each entry. The use of this is
left totally to the user. Lastly, the `Value` is a base64 key value.

If no entries are found, a 404 code is returned.

When using the `PUT` method, Consul expects the request body to be the
value corresponding to the key. There are a number of parameters that can
be used with a PUT request:

* ?flags=\<num\> : This can be used to specify an unsigned value between
  0 and 2^64-1. It is opaque to the user, but a client application may
  use it.

* ?cas=\<index\> : This flag is used to turn the `PUT` into a **Check-And-Set**
  operation. This is very useful as it allows clients to build more complex
  syncronization primitives on top. If the index is 0, then Consul will only
  put the key if it does not already exist. If the index is non-zero, then
  the key is only set if the index matches the `ModifyIndex` of that key.

The return value is simply either `true` or `false`. If the CAS check fails,
then `false` will be returned.

Lastly, the `DELETE` method can be used to delete a single key or all
keys sharing a prefix. If the "?recurse" query parameter is provided,
then all keys with the prefix are deleted, otherwise only the specified
key.


## Agent

The Agent endpoints are used to interact with a local Consul agent. Usually,
services and checks are registered with an agent, which then takes on the
burden of registering with the Catalog and performing anti-entropy to recover from
outages. There are also various control APIs that can be used instead of the
msgpack RPC protocol.

The following endpoints are supported:

* /v1/agent/checks: Returns the checks the local agent is managing
* /v1/agent/services : Returns the services local agent is managing
* /v1/agent/members : Returns the members as seen by the local serf agent
* /v1/agent/join/\<address\> : Trigger local agent to join a node
* /v1/agent/force-leave/\<node\>: Force remove node
* /v1/agent/check/register : Registers a new local check
* /v1/agent/check/deregister/\<checkID\> : Deregister a local check
* /v1/agent/check/pass/\<checkID\> : Mark a local test as passing
* /v1/agent/check/warn/\<checkID\> : Mark a local test as warning
* /v1/agent/check/fail/\<checkID\> : Mark a local test as critical
* /v1/agent/service/register : Registers a new local service
* /v1/agent/service/deregister/\<serviceID\> : Deregister a local service

### /v1/agent/checks

This endpoint is used to return the all the checks that are registered with
the local agent. These checks were either provided through configuration files,
or added dynamically using the HTTP API. It is important to note that the checks
known by the agent may be different than those reported by the Catalog. This is usually
due to changes being made while there is no leader elected. The agent performs active
anti-entropy, so in most situations everything will be in sync within a few seconds.

This endpoint is hit with a GET and returns a JSON body like this:

    {
        "service:redis":{
            "Node":"foobar",
            "CheckID":"service:redis",
            "Name":"Service 'redis' check",
            "Status":"passing",
            "Notes":"",
            "ServiceID":"redis",
            "ServiceName":"redis"
        }
    }

### /v1/agent/services

This endpoint is used to return the all the services that are registered with
the local agent. These services were either provided through configuration files,
or added dynamically using the HTTP API. It is important to note that the services
known by the agent may be different than those reported by the Catalog. This is usually
due to changes being made while there is no leader elected. The agent performs active
anti-entropy, so in most situations everything will be in sync within a few seconds.

This endpoint is hit with a GET and returns a JSON body like this:

    {
        "redis":{
            "ID":"redis",
            "Service":"redis",
            "Tag":"",
            "Port":8000
        }
    }

### /v1/agent/members

This endpoint is hit with a GET and returns the members the agent sees in the
cluster gossip pool. Due to the nature of gossip, this is eventually consistent
and the results may differ by agent. The strongly consistent view of nodes is
instead provided by "/v1/catalog/nodes".

For agents running in server mode, providing a "?wan=1" query parameter returns
the list of WAN members instead of the LAN members which is default.

This endpoint returns a JSON body like:

    [
        {
            "Name":"foobar",
            "Addr":"10.1.10.12",
            "Port":8301,
            "Tags":{
                "bootstrap":"1",
                "dc":"dc1",
                "port":"8300",
                "role":"consul"
            },
            "Status":1,
            "ProtocolMin":1,
            "ProtocolMax":2,
            "ProtocolCur":2,
            "DelegateMin":1,
            "DelegateMax":3,
            "DelegateCur":3
        }
    ]

### /v1/agent/join/\<address\>

This endpoint is hit with a GET and is used to instruct the agent to attempt to
connect to a given address.  For agents running in server mode, providing a "?wan=1"
query parameter causes the agent to attempt to join using the WAN pool.

The endpoint returns 200 on successful join.

### /v1/agent/force-leave/\<node\>

This endpoint is hit with a GET and is used to instructs the agent to force a node into the left state.
If a node fails unexpectedly, then it will be in a "failed" state. Once in this state, Consul will
attempt to reconnect, and additionally the services and checks belonging to that node will not be
cleaned up. Forcing a node into the left state allows its old entries to be removed.

The endpoint always returns 200.

### /v1/agent/check/register

The register endpoint is used to add a new check to the local agent.
There is more documentation on checks [here](/docs/agent/checks.html).
Checks are either a script or TTL type. The agent is reponsible for managing
the status of the check and keeping the Catalog in sync.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

    {
        "ID": "mem",
	    "Name": "Memory utilization",
	    "Notes": "Ensure we don't oversubscribe memory",
        "Script": "/usr/local/bin/check_mem.py",
        "Interval": "10s",
        "TTL": "15s"
    }

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

### /v1/agent/check/deregister/\<checkId\>

The deregister endpoint is used to remove a check from the local agent.
The CheckID must be passed after the slash. The agent will take care
of deregistering the check with the Catalog.

The return code is 200 on success.

### /v1/agent/check/pass/\<checkId\>

This endpoint is used with a check that is of the [TTL type](/docs/agent/checks.html).
When this endpoint is accessed, the status of the check is set to "passing", and
the TTL clock is reset.

The return code is 200 on success.

### /v1/agent/check/warn/\<checkId\>

This endpoint is used with a check that is of the [TTL type](/docs/agent/checks.html).
When this endpoint is accessed, the status of the check is set to "warning", and
the TTL clock is reset.

The return code is 200 on success.

### /v1/agent/check/fail/\<checkId\>

This endpoint is used with a check that is of the [TTL type](/docs/agent/checks.html).
When this endpoint is accessed, the status of the check is set to "critical", and
the TTL clock is reset.

The return code is 200 on success.

### /v1/agent/service/register

The register endpoint is used to add a new service to the local agent.
There is more documentation on services [here](/docs/agent/services.html).
Services may also provide a health check. The agent is reponsible for managing
the status of the check and keeping the Catalog in sync.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

    {
        "ID": "redis1",
	    "Name": "redis",
	    "Tag": "master",
	    "Port": 8000,
        "Check": {
            "Script": "/usr/local/bin/check_redis.py",
            "Interval": "10s",
            "TTL": "15s"
        }
    }

The `Name` field is mandatory,  If an `ID` is not provided, it is set to `Name`.
You cannot have duplicate `ID` entries per agent, so it may be necessary to provide an ID.
`Tag`, `Port` and `Check` are optional. If `Check` is provided, only one of `Script` and `Interval`
or `TTL` should be provided. There is more information about checks [here](/docs/agent/checks.html).

The created check will be named "service:\<ServiceId\>".

The return code is 200 on success.

### /v1/agent/service/deregister/\<serviceId\>

The deregister endpoint is used to remove a service from the local agent.
The ServiceID must be passed after the slash. The agent will take care
of deregistering the service with the Catalog. If there is an associated
check, that is also deregistered.

The return code is 200 on success.

## Catalog

The Catalog is the endpoint used to register and deregister nodes,
services, and checks. It also provides a number of query endpoints.

The following endpoints are supported:

* /v1/catalog/register : Registers a new node, service, or check
* /v1/catalog/deregister : Deregisters a node, service, or check
* /v1/catalog/datacenters : Lists known datacenters
* /v1/catalog/nodes : Lists nodes in a given DC
* /v1/catalog/services : Lists services in a given DC
* /v1/catalog/service/\<service\> : Lists the nodes in a given service
* /v1/catalog/node/\<node\> : Lists the services provided by a node

The last 4 endpoints of the catalog support blocking queries.

### /v1/catalog/register

The register endpoint is a low level mechanism for direclty registering
or updating entries in the catalog. It is usually recommended to use
the agent local endpoints, as they are simpler and perform anti-entropy.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

    {
        "Datacenter": "dc1",
        "Node": "foobar",
        "Address": "192.168.10.10",
        "Service": {
            "ID": "redis1",
            "Service": "redis",
            "Tag": "master",
            "Port": 8000,
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

The behavior of the endpoint depends on what keys are provided. The endpoint
requires `Node` and `Address` to be provided, while `Datacenter` will be defaulted
to match that of the agent. If only those are provided, the endpoint will register
the node with the catalog.

If the `Service` key is provided, then the service will also be registered. If
`ID` is not provided, it will be defaulted to `Service`. It is mandated that the
ID be node-unique. Both `Tag` and `Port` can be omitted.

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

### /v1/catalog/deregister

The deregister endpoint is a low level mechanism for direclty removing
entries in the catalog. It is usually recommended to use the agent local
endpoints, as they are simpler and perform anti-entropy.

The deregister endpoint expects a JSON request body to be PUT. The request
body must look like one of the following:

    {
        "Datacenter": "dc1",
        "Node": "foobar",
    }

    {
        "Datacenter": "dc1",
        "Node": "foobar",
        "CheckID": "service:redis1"
    }

    {
        "Datacenter": "dc1",
        "Node": "foobar",
        "ServiceID": "redis1",
    }

The behavior of the endpoint depends on what keys are provided. The endpoint
requires `Node` to be provided, while `Datacenter` will be defaulted
to match that of the agent. If only `Node` is provided, then the node, and
all associated services and checks are deleted. If `CheckID` is provided, only
that check belonging to the node is removed. If `ServiceID` is provided, then the
service along with it's associated health check (if any) is removed.

If the API call succeeds a 200 status code is returned.


### /v1/catalog/datacenters

This endpoint is hit with a GET and is used to return all the
datacenters that are known by the Consul server.

It returns a JSON body like this:

    ["dc1", "dc2"]

This endpoint does not require a cluster leader, and as such
will succeed even during an availability outage. It can thus be
a simple check to see if any Consul servers are routable.

### /v1/catalog/nodes

This endpoint is hit with a GET and returns the nodes known
about in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

    [
        {
            "Node":"baz",
            "Address":"10.1.10.11"
        },
        {
            "Node":"foobar",
            "Address":"10.1.10.12"
        }
    ]

This endpoint supports blocking queries.

### /v1/catalog/services

This endpoint is hit with a GET and returns the services known
about in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

    {
        "consul":[""],
        "redis":[""],
        "postgresql":["master","slave"]
    }

The main object keys are the service names, while the array
provides all the known tags for a given service.

This endpoint supports blocking queries.

### /v1/catalog/service/\<service\>

This endpoint is hit with a GET and returns the nodes providing a service
in a given DC. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

The service being queried must be provided after the slash. By default
all nodes in that service are returned. However, the list can be filtered
by tag using the "?tag=" query parameter.

It returns a JSON body like this:

    [
        {
            "Node":"foobar",
            "Address":"10.1.10.12",
            "ServiceID":"redis",
            "ServiceName":"redis",
            "ServiceTag":"",
            "ServicePort":8000
        }
    ]

This endpoint supports blocking queries.

### /v1/catalog/node/\<node\>

This endpoint is hit with a GET and returns the node provided services.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The node being queried must be provided after the slash.

It returns a JSON body like this:

    {
        "Node":{
            "Node":"foobar",
            "Address":"10.1.10.12"
        },
        "Services":{
            "consul":{
                "ID":"consul",
                "Service":"consul",
                "Tag":"",
                "Port":8300
            },
            "redis":{
                "ID":"redis",
                "Service":"redis",
                "Tag":"",
                "Port":8000
            }
        }
    }

This endpoint supports blocking queries.

## Health

The Health used to query health related information. It is provided seperately
from the Catalog, since users may prefer to not use the health checking mechanisms
as they are totally optional. Additionally, some of the query results from the Health system are filtered, while the Catalog endpoints provide the raw entries.

The following endpoints are supported:

* /v1/health/node/\<node\>: Returns the health info of a node
* /v1/health/checks/\<service\>: Returns the checks of a service
* /v1/health/service/\<service\>: Returns the nodes and health info of a service
* /v1/health/state/\<state\>: Returns the checks in a given state

All of the health endpoints supports blocking queries.

### /v1/health/node/\<node\>

This endpoint is hit with a GET and returns the node specific checks known.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The node being queried must be provided after the slash.

It returns a JSON body like this:

    [
        {
            "Node":"foobar",
            "CheckID":"serfHealth",
            "Name":"Serf Health Status",
            "Status":"passing",
            "Notes":"",
            "ServiceID":"",
            "ServiceName":""
        },
        {
            "Node":"foobar",
            "CheckID":"service:redis",
            "Name":"Service 'redis' check",
            "Status":"passing",
            "Notes":"",
            "ServiceID":"redis",
            "ServiceName":"redis"
        }
    ]

In this case, we can see there is a system level check (no associated
`ServiceID`, as well as a service check for Redis). The "serfHealth" check
is special, in that all nodes automatically have this check. When a node
joins the Consul cluster, it is part of a distributed failure detection
provided by Serf. If a node fails, it is detected and the status is automatically
changed to "critical".

This endpoint supports blocking queries.

### /v1/health/checks/\<service\>

This endpoint is hit with a GET and returns the checks associated with
a service in a given datacenter.
By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.
The service being queried must be provided after the slash.

It returns a JSON body like this:

    [
        {
            "Node":"foobar",
            "CheckID":"service:redis",
            "Name":"Service 'redis' check",
            "Status":"passing",
            "Notes":"",
            "ServiceID":"redis",
            "ServiceName":"redis"
        }
    ]

This endpoint supports blocking queries.

### /v1/health/service/\<service\>

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

Users can also built in support for dynamic load balancing and other features
by incorporating the use of health checks.

It returns a JSON body like this:

    [
        {
            "Node":{
                "Node":"foobar",
                "Address":"10.1.10.12"
            },
            "Service":{
                "ID":"redis",
                "Service":"redis",
                "Tag":"",
                "Port":8000
            },
            "Checks":[
                {
                    "Node":"foobar",
                    "CheckID":"service:redis",
                    "Name":"Service 'redis' check",
                    "Status":"passing",
                    "Notes":"",
                    "ServiceID":"redis",
                    "ServiceName":"redis"
                },{
                    "Node":"foobar",
                    "CheckID":"serfHealth",
                    "Name":"Serf Health Status",
                    "Status":"passing",
                    "Notes":"",
                    "ServiceID":"",
                    "ServiceName":""
                }
            ]
        }
    ]

This endpoint supports blocking queries.

### /v1/health/state/\<state\>

This endpoint is hit with a GET and returns the checks in a specific
state for a given datacenter. By default the datacenter of the agent is queried,
however the dc can be provided using the "?dc=" query parameter.

The state being queried must be provided after the slash. The supported states
are "unknown", "passing", "warning", or "critical".

It returns a JSON body like this:

    [
        {
            "Node":"foobar",
            "CheckID":"serfHealth",
            "Name":"Serf Health Status",
            "Status":"passing",
            "Notes":"",
            "ServiceID":"",
            "ServiceName":""
        },
        {
            "Node":"foobar",
            "CheckID":"service:redis",
            "Name":"Service 'redis' check",
            "Status":"passing",
            "Notes":"",
            "ServiceID":"redis",
            "ServiceName":"redis"
        }
    ]

This endpoint supports blocking queries.

## Status

The Status endpoints are used to get information about the status
of the Consul cluster. This are generally very low level, and not really
useful for clients.

The following endpoints are supported:

* /v1/status/leader : Returns the current Raft leader
* /v1/status/peers : Returns the current Raft peer set

### /v1/status/leader

This endpoint is used to get the Raft leader for the datacenter
the agent is running in. It returns only an address like:

    "10.1.10.12:8300"

### /v1/status/peers

This endpoint is used to get the Raft peers for the datacenter
the agent is running in. It returns a list of addresses like:

    ["10.1.10.12:8300", "10.1.10.11:8300", "10.1.10.10:8300"]


