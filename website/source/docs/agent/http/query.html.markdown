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
would be possible given the limited entry points exposed by DNS.

Consul 0.6.4 and later also supports prepared query templates. Templates are
defined in a similar way to regular prepared queries but instead of applying to
just a single query name, they can respond to names starting with a configured
prefix. The service name being queried is computed using the matched prefix
and/or a regular expression. This provides a powerful tool that lets you apply
the features of prepared queries to a range (or potentially all) services with a
small number of templates. Details about prepared query templates are covered
[below](#templates).

The following endpoints are supported:

* [`/v1/query`](#general): Creates a new prepared query or lists
  all prepared queries
* [`/v1/query/<query>`](#specific): Updates, fetches, or deletes
  a prepared query
* [`/v1/query/<query or name>/execute`](#execute): Executes a
  prepared query by its ID or optional name
* [`/v1/query/<query or name>/explain`](#explain): Provides information about
  how a prepared query will be executed by its ID or optional name

Not all endpoints support blocking queries and all consistency modes,
see details in the sections below.

The query endpoints support the use of ACL Tokens. Prepared queries have some
special handling of ACL Tokens that are called out where applicable with the
details of each endpoint.

See the [Prepared Query ACLs](/docs/internals/acl.html#prepared_query_acls)
internals guide for more details about how prepared query policies work.

### <a name="general"></a> /v1/query

The general query endpoint supports the `POST` and `GET` methods.

#### POST Method

When using the `POST` method, Consul will create a new prepared query and return
its ID if it is created successfully.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with `query`
write privileges for the `Name` of the query being created.

The create operation expects a JSON request body that defines the prepared
query, like this example:

```javascript
{
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

<a name="token"></a>
`Token`, if specified, is a captured ACL Token that is reused as the ACL Token
every time the query is executed. This allows queries to be executed by clients
with lesser or even no ACL Token, so this should be used with care. The token
itself can only be seen by clients with a management token. If the `Token`
field is left blank or omitted, the client's ACL Token will be used to determine
if they have access to the service being queried. If the client does not supply
an ACL Token, the anonymous token will be used.

Note that Consul version 0.6.3 and earlier would automatically capture the ACL
Token for use in the future when prepared queries were executed and would
execute with the same privileges as the definer of the prepared query. Older
queries wishing to obtain the new behavior will need to be updated to remove
their captured `Token` field. Capturing ACL Tokens is analogous to
[PostgreSQL’s SECURITY DEFINER](http://www.postgresql.org/docs/current/static/sql-createfunction.html)
attribute which can be set on functions. This change in effect moves Consul
from using `SECURITY DEFINER` by default to `SECURITY INVOKER` by default for
new Prepared Queries.

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

<a name="templates"><b>Prepared Query Templates</b></a>
Consul 0.6.4 and later also support prepared query templates. These are created similar
to static templates, except with some additional fields and features. Here's an example:

```javascript
{
  "Name": "geo-db",
  "Template": {
    "Type": "name_prefix_match",
    "Regexp": "^geo-db-(.*?)-([^\\-]+?)$"
  },
  "Service": {
    "Service": "mysql-${match(1)}",
    "Failover": {
      "NearestN": 3,
      "Datacenters": ["dc1", "dc2"]
    },
    "OnlyPassing": true,
    "Tags": ["${match(2)}"]
  }
}
```

The new `Template` structure configures a prepared query as a template instead of a
static query. It has two fields:

`Type` is the query type, which must be "name_prefix_match". This means that the
template will apply to any query lookup with a name whose prefix matches the `Name`
field of the template. In this example, any query for "geo-db" will match this
query. Query templates are resolved using a longest prefix match, so it's possible
to have high-level templates that are overridden for specific services. Static
queries are always resolved first, so they can also override templates.

`Regexp` is an optional regular expression which is used to extract fields from the
entire name, once this template is selected. In this example, the regular expression
takes the first item after the "-" as the database name and everything else after as
a tag. See the [RE2](https://github.com/google/re2/wiki/Syntax) reference for syntax
of this regular expression.

All other fields of the query have the same meanings as for a static query, except
that several interpolation variables are available to dynamically populate the query
before it is executed. All of the string fields inside the `Service` structure are
interpolated, with the following variables available:

`${name.full}` has the entire name that was queried. For example, a DNS lookup for
"geo-db-customer-master.query.consul" in the example above would set this variable to
"geo-db-customer-master".

`${name.prefix}` has the prefix that matched. This would always be "geo-db" for
the example above.

`${name.suffix}` has the suffix after the prefix. For example, a DNS lookup for
"geo-db-customer-master.query.consul" in the example above would set this variable to
"-customer-master".

`${match(N)}` returns the regular expression match at the given index N. The
0 index will have the entire match, and >0 will have the results of each match
group. For example, a DNS lookup for "geo-db-customer-master.query.consul" in the example
above with a `Regexp` field set to `^geo-db-(.*?)-([^\-]+?)$` would return
"geo-db-customer-master" for `${match(0)}`, "customer" for `${match(1)}`, and
"master" for `${match(2)}`. If the regular expression doesn't match, or an invalid
index is given, then `${match(N)}` will return an empty string.

See the [query explain](#explain) endpoint which is useful for testing interpolations
and determining which query is handling a given name.

Using templates it's possible to apply prepared query behaviors to many services
with a single template. Here's an example template that matches any query and
applies a failover policy to it:

```javascript
{
	"Name": "",
	"Template": {
		"Type": "name_prefix_match"
	},
	"Service": {
		"Service": "${name.full}",
		"Failover": {
			"NearestN": 3
		}
	}
}
```

This will match any lookup for `*.query.consul` and will attempt to find the
service locally, and otherwise attempt to find that service in the next three
closest datacenters. If ACLs are enabled, a catch-all template like this with
an empty `Name` requires an ACL token that can write to any query prefix. Also,
only a single catch-all template can be registered at any time.

#### GET Method

When using the GET method, Consul will provide a listing of all prepared queries.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter. This endpoint supports blocking
queries and all consistency modes.

If ACLs are enabled, then the client will only see prepared queries for which their
token has `query` read privileges. A management token will be able to see all
prepared queries. Tokens will be redacted and displayed as `<hidden>` unless a
management token is used.

This returns a JSON list of prepared queries, which looks like:

```javascript
[
  {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "Name": "my-query",
    "Session": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "Token": "<hidden>",
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

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with `query`
write privileges for the `Name` of the query being updated.

The body is the same as is used to create a prepared query, as described above.

If the API call succeeds, a 200 status code is returned.

#### GET Method

The `GET` method allows an existing prepared query to be fetched.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter. This endpoint supports blocking
queries and all consistency modes.

The returned response is the same as the list of prepared queries above,
only with a single item present. If the query does not exist then a 404
status code will be returned.

If ACLs are enabled, then the client will only see prepared queries for which their
token has `query` read privileges. A management token will be able to see all
prepared queries. Tokens will be redacted and displayed as `<hidden>` unless a
management token is used.

#### DELETE Method

The `DELETE` method is used to delete a prepared query.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter.

If ACLs are enabled, the client will need to supply an ACL Token with `query`
write privileges for the `Name` of the query being deleted.

No body is required as part of this request.

If the API call succeeds, a 200 status code is returned.

### <a name="execute"></a> /v1/query/\<query or name\>/execute

The query execute endpoint supports only the `GET` method and is used to
execute a prepared query. The \<query or name\> argument is the ID or name
of an existing prepared query, or a name that matches a prefix name for a
[prepared query template](#templates).

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter. This endpoint does not support
blocking queries, but it does support all consistency modes.

Adding the optional "?near=" parameter with a node name will sort the resulting
list in ascending order based on the estimated round trip time from that node.
Passing "?near=_agent" will use the agent's node for the sort. If this is not
present, then the nodes will be shuffled randomly and will be in a different
order each time the query is executed.

An optional "?limit=" parameter can be used to limit the size of the list to
the given number of nodes. This is applied after any sorting or shuffling.

If an ACL Token was bound to the query when it was defined then it will be used
when executing the request. Otherwise, the client's supplied ACL Token will be
used.

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
  }]
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

### <a name="explain"></a> /v1/query/\<query or name\>/explain

The query explain endpoint supports only the `GET` method and is used to see
a fully-rendered query for a given name. This is especially useful for finding
which [prepared query template](#templates) matches a given name, and what the
final query looks like after interpolation.

By default, the datacenter of the agent is queried; however, the `dc` can be
provided using the "?dc=" query parameter. This endpoint does not support
blocking queries, but it does support all consistency modes.

If ACLs are enabled, then the client will only see prepared queries for which their
token has `query` read privileges. A management token will be able to see all
prepared queries. Tokens will be redacted and displayed as `<hidden>` unless a
management token is used.

If the query does not exist then a 404 status code will be returned. Otherwise,
a JSON body will be returned like this:

```javascript
{
  "Query": {
    "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05",
    "Name": "my-query",
    "Session": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "Token": "<hidden>",
    "Name": "geo-db",
    "Template": {
      "Type": "name_prefix_match",
      "Regexp": "^geo-db-(.*?)-([^\\-]+?)$"
    },
    "Service": {
      "Service": "mysql-customer",
      "Failover": {
        "NearestN": 3,
        "Datacenters": ["dc1", "dc2"]
      },
      "OnlyPassing": true,
      "Tags": ["master"]
    }
}
```

Note that even though this query is a template, it is shown with its `Service`
fields interpolated based on the example query name "geo-db-customer-master".
