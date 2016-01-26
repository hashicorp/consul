---
layout: "docs"
page_title: "Prepared Queries (HTTP)"
sidebar_current: "docs-agent-http-query"
description: >
  The Query endpoints are used to manage and execute prepared queries.
---

# Prepared Query HTTP Endpoint

The Prepared Query endpoints are used to create, update, destroy, and execute
prepared queries.

Prepared queries allow you to register a complex service query and then execute
it later via its ID or name to get a set of healthy nodes that provide a given
service. This is particularly useful in combination with Consul's
[DNS Interface](/docs/agent/dns.html) as it allows for much richer queries than
would be possible given the limited interface DNS provides.

The following endpoints are supported:

* [`/v1/query`](#general): Creates a new prepared query or lists
  all prepared queries
* [`/v1/query/<query>`](#specific): Updates, fetches, or deletes
  a prepared query
* [`/v1/query/<query or name>/execute`](#execute): Executes a
  prepared query by its ID or optional name

Not all endpoints support blocking queries and all consistency modes,
see details in the sections below.

The query endpoints support the use of ACL tokens. Prepared queries have some
special handling of ACL tokens that are highlighted in the sections below.

### <a name="general"></a> /v1/query

The general query endpoint supports the `POST` and `GET` methods.

#### POST Method

When using the `POST` method, Consul will create a new prepared query and return
its ID if it is created successfully.

By default, the datacenter of the agent is queried; however, the dc can be
provided using the "?dc=" query parameter.

The create operation expects a JSON request body that defines the prepared query,
like this example:

```javascript
{
  "Name": "my-query",
  "Session": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
  "Service": {
    "Service": "redis",
    "Failover": {
      "NearestN": 3,
      "Datacenters": ["dc1", "dc2"]
    },
    "OnlyPassing": false,
    "Tags": ["master", "!experimental"]
  },
  "DNS": {
    "TTL": "10s"
  }
}
```

Only the `Service` field inside the `Service` structure is mandatory, all other
fields will take their default values if they are not included.

`Name` is an optional friendly name that can be used to execute a query instead
of using its ID.

`Session` provides a way to automatically remove a prepared query when the
given session is invalidated. This is optional, and if not given the prepared
query must be manually removed when no longer needed.

The set of fields inside the `Service` structure define the query's behavior.

`Service` is the name of the service to query. This is required.

`Failover` contains two fields, both of which are optional, and determine what
happens if no healthy nodes are available in the local datacenter when the query
is executed. It allows the use of nodes in other datacenters with very little
configuration.

If `NearestN` is set to a value greater than zero, then the query
will be forwarded to up to `NearestN` other datacenters based on their estimated
network round trip time using [Network Coordinates](/docs/internals/coordinates.html)
from the WAN gossip pool. The median round trip time from the server handling the
query to the servers in the remote datacenter is used to determine the priority.
The default value is zero. All Consul servers must be running version 0.6.0 or
above in order for this feature to work correctly. If any servers are not running
the required version of Consul they will be considered last since they won't have
any available network coordinate information.

`Datacenters` contains a fixed list of remote datacenters to forward the query
to if there are no healthy nodes in the local datacenter. Datacenters are queried
in the order given in the list. If this option is combined with `NearestN`, then
the `NearestN` queries will be performed first, followed by the list given by
`Datacenters`. A given datacenter will only be queried one time during a failover,
even if it is selected by both `NearestN` and is listed in `Datacenters`. The
default value is an empty list.

`OnlyPassing` controls the behavior of the query's health check filtering. If
this is set to false, the results will include nodes with checks in the passing
as well as the warning states. If this is set to true, only nodes with checks
in the passing state will be returned. The default value is false.

`Tags` provides a list of service tags to filter the query results. For a service
to pass the tag filter it must have *all* of the required tags, and *none* of the
excluded tags (prefixed with `!`). The default value is an empty list, which does
no tag filtering.

`TTL` in the `DNS` structure is a duration string that can use "s" as a
suffix for seconds. It controls how the TTL is set when query results are served
over DNS. If this isn't specified, then the Consul agent configuration for the given
service will be used (see [DNS Caching](/docs/guides/dns-cache.html)). If this is
specified, it will take precedence over any Consul agent-specific configuration.
If no TTL is specified here or at the Consul agent level, then the TTL will
default to 0.

The return code is 200 on success and the ID of the created query is returned in
a JSON body:

```javascript
{
  "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05"
}
```

If ACLs are enabled, then the provided token will be used to check access to
the service being queried, and it will be saved along with the query for use
when the query is executed. This is key to allowing prepared queries to work
via the DNS interface, and it's important to note that prepared query IDs and
names become a read-only proxy for the token used to create the query.

The query IDs that Consul generates are done in the same manner as ACL tokens,
so provide equal strength, but names may be more guessable and should be used
carefully with ACLs. Also, the token used to create the prepared query (or a
management token) is required to read the query back, so the ability to execute
a prepared query is not enough to get access to the actual token.

#### GET Method

When using the GET method, Consul will provide a listing of all prepared queries.

By default, the datacenter of the agent is queried; however, the dc can be
provided using the "?dc=" query parameter. This endpoint supports blocking
queries and all consistency modes.

Since this listing includes sensitive ACL tokens, this is a privileged endpoint
and always requires a management token to be supplied if ACLs are enabled.

This returns a JSON list of prepared queries, which looks like:

```javascript
[
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "Name": "my-query",
    "Session": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "Token": "",
    "Service": {
      "Service": "redis",
      "Failover": {
        "NearestN": 3,
        "Datacenters": ["dc1", "dc2"]
      },
      "OnlyPassing": false,
      "Tags": ["master", "!experimental"]
    },
    "DNS": {
      "TTL": "10s"
    },
    "RaftIndex": {
      "CreateIndex": 23,
      "ModifyIndex": 42
    }
  }
]
```

### <a name="specific"></a> /v1/query/\<query\>

The query-specific endpoint supports the `GET`, `PUT`, and `DELETE` methods. The
\<query\> argument is the ID of an existing prepared query.

#### PUT Method

The `PUT` method allows an existing prepared query to be updated.

By default, the datacenter of the agent is queried; however, the dc can be
provided using the "?dc=" query parameter.

If ACLs are enabled, then the same token used to create the query (or a
management token) must be supplied.

The body is the same as is used to create a prepared query, as described above.

If the API call succeeds, a 200 status code is returned.

#### GET Method

The `GET` method allows an existing prepared query to be fetched.

By default, the datacenter of the agent is queried; however, the dc can be
provided using the "?dc=" query parameter. This endpoint supports blocking
queries and all consistency modes.

The returned response is the same as the list of prepared queries above,
only with a single item present. If the query does not exist then a 404
status code will be returned.

If ACLs are enabled, then the same token used to create the query (or a
management token) must be supplied.

#### DELETE Method

The `DELETE` method is used to delete a prepared query.

By default, the datacenter of the agent is queried; however, the dc can be
provided using the "?dc=" query parameter.

If ACLs are enabled, then the same token used to create the query (or a
management token) must be supplied.

No body is required as part of this request.

If the API call succeeds, a 200 status code is returned.

### <a name="execute"></a> /v1/query/\<query or name\>/execute

The query execute endpoint supports only the `GET` method and is used to
execute a prepared query. The \<query or name\> argument is the ID or name
of an existing prepared query.

By default, the datacenter of the agent is queried; however, the dc can be
provided using the "?dc=" query parameter. This endpoint does not support
blocking queries, but it does support all consistency modes.

Adding the optional "?near=" parameter with a node name will sort the resulting
list in ascending order based on the estimated round trip time from that node.
Passing "?near=_agent" will use the agent's node for the sort. If this is not
present, then the nodes will be shuffled randomly and will be in a different
order each time the query is executed.

An optional "?limit=" parameter can be used to limit the size of the list to
the given number of nodes. This is applied after any sorting or shuffling.

The ACL token supplied when the prepared query was created will be used to
execute the request, so no ACL token needs to be supplied (it will be ignored).

No body is required as part of this request.

If the query does not exist then a 404 status code will be returned. Otherwise,
a JSON body will be returned like this:

```javascript
{
  "Service": "redis",
  "Nodes": [
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
      ],
    "DNS": {
      "TTL": "10s"
    },
    "Datacenter": "dc3",
    "Failovers": 2
  }
}
```

The `Nodes` section contains the list of healthy nodes providing the given
service, as specified by the constraints of the prepared query.

`Service` has the service name that the query was selecting. This is useful
for context in case an empty list of nodes is returned.

`DNS` has information used when serving the results over DNS. This is just a
copy of the structure given when the prepared query was created.

`Datacenter` has the datacenter that ultimately provided the list of nodes
and `Failovers` has the number of remote datacenters that were queried
while executing the query. This provides some insight into where the data
came from. This will be zero during non-failover operations where there
were healthy nodes found in the local datacenter.
