---
layout: "docs"
page_title: "Sessions (HTTP)"
sidebar_current: "docs-agent-http-sessions"
description: >
  The Session endpoints are used to create, destroy, and query sessions.
---

# Session HTTP Endpoint

The Session endpoints are used to create, destroy, and query sessions.
The following endpoints are supported:

* [`/v1/session/create`](#session_create): Creates a new session
* [`/v1/session/destroy/<session>`](#session_destroy): Destroys a given session
* [`/v1/session/info/<session>`](#session_info): Queries a given session
* [`/v1/session/node/<node>`](#session_node): Lists sessions belonging to a node
* [`/v1/session/list`](#session_list): Lists all active sessions
* [`/v1/session/renew`](#session_renew): Renews a TTL-based session

All of the read session endpoints support blocking queries and all consistency modes.

### <a name="session_create"></a> /v1/session/create

The create endpoint is used to initialize a new session.
There is more documentation on sessions [here](/docs/internals/sessions.html).
Sessions must be associated with a node and may be associated with any number of checks.

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
if the defaults are to be used.

By default, the agent's local datacenter is used; another datacenter
can be specified using the "?dc=" query parameter. However, it is not recommended
to use cross-datacenter sessions.

`LockDelay` can be specified as a duration string using a "s" suffix for
seconds. The default is 15s.

`Node` must refer to a node that is already registered, if specified. By default,
the agent's own node name is used.

`Name` can be used to provide a human-readable name for the Session.

`Checks` is used to provide a list of associated health checks. It is highly recommended
that, if you override this list, you include the default "serfHealth".

`Behavior` can be set to either `release` or `delete`. This controls
the behavior when a session is invalidated. By default, this is `release`, 
causing any locks that are held to be released. Changing this to `delete`
causes any locks that are held to be deleted. `delete` is useful for creating ephemeral
key/value entries.

The `TTL` field is a duration string, and like `LockDelay` it can use "s" as
a suffix for seconds. If specified, it must be between 10s and 86400s currently.
When provided, the session is invalidated if it is not renewed before the TTL
expires. The lowest practical TTL should be used to keep the number of managed
sessions low. When locks are forcibly expired, such as during a leader election,
sessions may not be reaped for up to double this TTL, so long TTL values (>1 hour)
should be avoided. See the [session internals page](/docs/internals/sessions.html)
for more documentation of this feature.

The return code is 200 on success and returns the ID of the created session:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

### <a name="session_destroy"></a> /v1/session/destroy/\<session\>

The destroy endpoint is hit with a PUT and destroys the given session.
By default, the local datacenter is used, but the "?dc=" query parameter
can be used to specify the datacenter.

The session being destroyed must be provided on the path.

The return code is 200 on success.

### <a name="session_info"></a> /v1/session/info/\<session\>

This endpoint is hit with a GET and returns the requested session information
within a given datacenter. By default, the datacenter of the agent is queried;
however, the dc can be provided using the "?dc=" query parameter.
The session being queried must be provided on the path.

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
for a given node and datacenter. By default, the datacenter of the agent is queried;
however, the dc can be provided using the "?dc=" query parameter.

The node being queried must be provided on the path.

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
for a given datacenter. By default, the datacenter of the agent is queried;
however, the dc can be provided using the "?dc=" query parameter.

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
This is used with sessions that have a TTL, and it extends the
expiration by the TTL. By default, the local datacenter is used, but the "?dc="
query parameter can be used to specify the datacenter.

The session being renewed must be provided on the path.

The return code is 200 on success.  The response JSON body looks like this:

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

Note: Consul MAY return a TTL value higher than the one specified during session creation.
This indicates the server is under high load and is requesting clients renew less
often.
