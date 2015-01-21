---
layout: "docs"
page_title: "Agent (HTTP)"
sidebar_current: "docs-agent-http-agent"
description: >
  The Agent endpoints are used to interact with a local Consul agent.
---

# Agent HTTP Endpoint

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
* [`/v1/agent/maintenance`](#agent_maintenance) : Node maintenance mode
* [`/v1/agent/join/<address>`](#agent_join) : Trigger local agent to join a node
* [`/v1/agent/force-leave/<node>`](#agent_force_leave)>: Force remove node
* [`/v1/agent/check/register`](#agent_check_register) : Registers a new local check
* [`/v1/agent/check/deregister/<checkID>`](#agent_check_deregister) : Deregister a local check
* [`/v1/agent/check/pass/<checkID>`](#agent_check_pass) : Mark a local test as passing
* [`/v1/agent/check/warn/<checkID>`](#agent_check_warn) : Mark a local test as warning
* [`/v1/agent/check/fail/<checkID>`](#agent_check_fail) : Mark a local test as critical
* [`/v1/agent/service/register`](#agent_service_register) : Registers a new local service
* [`/v1/agent/service/deregister/<serviceID>`](#agent_service_deregister) : Deregister a local service
* [`/v1/agent/service/maintenance/<serviceID>`](#agent_service_maintenance) : Service maintenance mode

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
    "Address": "",
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

### <a name="agent_maintenance"></a> /v1/agent/maintenance

The node maintenance endpoint allows placing the agent into "maintenance mode".
During maintenance mode, the node will be marked as unavailable, and will not be
present in DNS or API queries. This API call is idempotent. Maintenance mode is
persistent and will be automatically restored on agent restart.

The `?enable` flag is required, and its value must be `true` (to enter
maintenance mode), or `false` (to resume normal operation).

The `?reason` flag is optional, and can contain a text string explaining the
reason for placing the node into maintenance mode. If no reason is provided,
a default value will be used instead.

The return code is 200 on success.

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
Checks are of script, HTTP, or TTL type. The agent is responsible for managing
the status of the check and keeping the Catalog in sync.

The register endpoint expects a JSON request body to be PUT. The request
body must look like:

```javascript
{
  "ID": "mem",
  "Name": "Memory utilization",
  "Notes": "Ensure we don't oversubscribe memory",
  "Script": "/usr/local/bin/check_mem.py",
  "HTTP": "http://example.com",
  "Interval": "10s",
  "TTL": "15s"
}
```

The `Name` field is mandatory, as is one of `Script`, `HTTP` or `TTL`.
`Script` and `HTTP` also require that `Interval` be set.

If an `ID` is not provided, it is set to `Name`. You cannot have duplicate
`ID` entries per agent, so it may be necessary to provide an ID. The `Notes`
field is not used by Consul, and is meant to be human readable.

If a `Script` is provided, the check type is a script, and Consul will
evaluate the script every `Interval` to update the status.

An `HTTP` check will preform an HTTP GET request to the value of `HTTP` (expected to be a URL) every `Interval`. If the response is any `2xx` code the check is passing, if the response is `429 Too Many Requests` the check is warning, otherwise the check is critical.

If a `TTL` type is used, then the TTL update APIs must be used to periodically update
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
  "Address": "127.0.0.1",
  "Port": 8000,
  "Check": {
    "Script": "/usr/local/bin/check_redis.py",
    "HTTP": "http://localhost:5000/health",
    "Interval": "10s",
    "TTL": "15s"
  }
}
```

The `Name` field is mandatory,  If an `ID` is not provided, it is set to `Name`.
You cannot have duplicate `ID` entries per agent, so it may be necessary to provide an ID.
`Tags`, `Address`, `Port` and `Check` are optional.
If `Check` is provided, only one of `Script`, `HTTP` or `TTL` should be provided.
`Script` and `HTTP` also require `Interval`.
There is more information about checks [here](/docs/agent/checks.html).
The `Address` will default to that of the agent if not provided.

The created check will be named "service:\<ServiceId\>".

The return code is 200 on success.

### <a name="agent_service_deregister"></a> /v1/agent/service/deregister/\<serviceId\>

The deregister endpoint is used to remove a service from the local agent.
The ServiceID must be passed after the slash. The agent will take care
of deregistering the service with the Catalog. If there is an associated
check, that is also deregistered.

The return code is 200 on success.

### <a name="agent_service_maintenance"></a> /v1/agent/service/maintenance/\<serviceId\>

The service maintenance endpoint allows placing a given service into
"maintenance mode". During maintenance mode, the service will be marked as
unavailable, and will not be present in DNS or API queries. This API call is
idempotent. Maintenance mode is persistent and will be automatically restored
on agent restart.

The `?enable` flag is required, and its value must be `true` (to enter
maintenance mode), or `false` (to resume normal operation).

The `?reason` flag is optional, and can contain a text string explaining the
reason for placing the service into maintenance mode. If no reason is provided,
a default value will be used instead.

The return code is 200 on success.
