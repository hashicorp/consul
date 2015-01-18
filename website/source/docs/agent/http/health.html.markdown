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

## <a name="session"></a> Session

The Session endpoints are used to create, destroy and query sessions.
The following endpoints are supported:

* [`/v1/session/create`](#session_create): Creates a new session
* [`/v1/session/destroy/<session>`](#session_destroy): Destroys a given session
* [`/v1/session/info/<session>`](#session_info): Queries a given session
* [`/v1/session/node/<node>`](#session_node): Lists sessions belonging to a node
* [`/v1/session/list`](#session_list): Lists all the active sessions
* [`/v1/session/renew`](#session_renew): Renew a TTL based session

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
  "Checks": ["a", "b", "c"],
  "Behavior": "release",
  "TTL": "0s"
}
```

None of the fields are mandatory, and in fact no body needs to be PUT
if the defaults are to be used. The `LockDelay` field can be specified
as a duration string using a "s" suffix for seconds. It can also be a numeric
value. Small values are treated as seconds, and otherwise it is provided with
nanosecond granularity.

The `Node` field must refer to a node that is already registered. By default,
the agent will use it's own name. The `Name` field can be used to provide a human
readable name for the Session. The `Checks` field is used to provide
a list of associated health checks. By default the "serfHealth" check is provided.
It is highly recommended that if you override this list, you include that check.

The `Behavior` field can be set to either "release" or "delete". This controls
the behavior when a session is invalidated. By default this is "release", and
this causes any locks that are held to be released. Changing this to "delete"
causes any locks that are held to be deleted. This is useful to create ephemeral
key/value entries.

The `TTL` field is a duration string, and like `LockDelay` it can use "s" as
a suffix for seconds. If specified, it must be between 10s and 3600s currently.
When provided, the session is invalidated if it is not renewed before the TTL
expires. See the [session internals page](/docs/internals/session.html) for more documentation.

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

### <a name="session_renew"></a> /v1/session/renew/\<session\>

The renew endpoint is hit with a PUT and renews the given session.
This is used with sessions that have a TTL set, and it extends the
expiration by the TTL. By default the local datacenter is used, but the "?dc="
query parameter can be used to specify the datacenter. The session being renewed
must be provided after the slash.

The return code is 200 on success and a JSON body like this:

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
    "Behavior": "release",
    "TTL": "15s"
  }
]
```

The response body includes the current session.
Consul MAY return a TTL value higher than the one specified during session creation.
This indicates the server is under high load and is requesting clients renew less
often.


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

To support [watches](/docs/agent/watches.html), this endpoint supports
blocking queries. However, the semantics of this endpoint are slightly
different. Most blocking queries provide a monotonic index, and block
until a newer index is available. This can be supported as a consequence
of the total ordering of the [consensus protocol](/docs/internals/consensus.html).
With gossip, there is no ordering, and instead `X-Consul-Index` maps
to the newest event that matches the query.

In practice, this means the index is only useful when used against a
single agent, and has no meaning globally. Because Consul defines
the index as being opaque, clients should not be expecting a natural
ordering either.

Agents only buffer the most recent entries. The number of entries should
not be depended upon, but currently defaults to 256. This value could
change in the future. The buffer should be large enough for most clients
and watches.

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
