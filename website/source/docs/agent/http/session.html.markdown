---
layout: "docs"
page_title: "Sessions (HTTP)"
sidebar_current: "docs-agent-http-sessions"
description: >
  The Session endpoints are used to create, destroy and query sessions.
---

# Session HTTP Endpoint

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
