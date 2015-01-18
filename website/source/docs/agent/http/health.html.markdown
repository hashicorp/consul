---
layout: "docs"
page_title: "Health Checks (HTTP)"
sidebar_current: "docs-agent-http-health"
description: >
  The Health used to query health related information.
---

# Health HTTP Endpoint

The Health used to query health related information. It is provided separately
from the Catalog, since users may prefer to not use the health checking mechanisms
as they are totally optional. Additionally, some of the query results from the
Health system are filtered, while the Catalog endpoints provide the raw entries.

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
