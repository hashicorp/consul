---
layout: "docs"
page_title: "HTTP API"
sidebar_current: "docs-agent-http"
---

# HTTP API

The main interface to Consul is a RESTful HTTP API. The API can be
used for CRUD for nodes, services, and checks. The endpoints are
versioned to enable changes without breaking backwards compatibility.

All endpoints fall into one of 4 categories:

* catalog - Manages nodes and services
* health - Manages health checks
* agent - Manages agent local state
* status - Consul system status

Each of the categories and their respective endpoints are documented below.

## Catalog

The Catalog is the major endpoint, as it is used to register and
deregister nodes, services, and checks. It also provides a number of
query endpoints.

The following endpoints are supported:

* /v1/catalog/register : Registers a new node, service, or check
* /v1/catalog/deregister : Deregisters a node, service, or check
* /v1/catalog/datacenters : Lists known datacenters
* /v1/catalog/nodes : Lists nodes in a given DC
* /v1/catalog/services : Lists services in a given DC
* /v1/catalog/service/<service>/ : Lists the nodes in a given service
* /v1/catalog/node/<node>/ : Lists the services provided by a node

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

### /v1/catalog/service/<service>

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

### /v1/catalog/node/<node>

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

## Health

The Health used to query health related information. It is provided seperately
from the Catalog, since users may prefer to not use the health checking mechanisms
as they are totally optional. Additionally, some of the query results from the Health system are filtered, while the Catalog endpoints provide the raw entries.

The following endpoints are supported:

* /v1/health/node/<node>: Returns the health info of a node
* /v1/health/checks/<service>: Returns the checks of a service
* /v1/health/service/<service>: Returns the nodes and health info of a service
* /v1/health/state/<state>: Returns the checks in a given state

### /v1/health/node/<node>

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

### /v1/health/checks/<service>

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

### /v1/health/service/<service>

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

### /v1/health/state/<state>

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

