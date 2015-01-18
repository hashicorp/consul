---
layout: "docs"
page_title: "ACLs (HTTP)"
sidebar_current: "docs-agent-http-acl"
description: >
  The ACL endpoints are used to create, update, destroy and query ACL tokens.
---

# ACL HTTP Endpoint

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
