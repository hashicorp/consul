---
layout: "docs"
page_title: "ACLs (HTTP)"
sidebar_current: "docs-agent-http-acl"
description: >
  The ACL endpoints are used to create, update, destroy, and query ACL tokens.
---

# ACL HTTP Endpoint

The ACL endpoints are used to create, update, destroy, and query ACL tokens.
The following endpoints are supported:

* [`/v1/acl/create`](#acl_create): Creates a new token with a given policy
* [`/v1/acl/update`](#acl_update): Updates the policy of a token
* [`/v1/acl/destroy/<id>`](#acl_destroy): Destroys a given token
* [`/v1/acl/info/<id>`](#acl_info): Queries the policy of a given token
* [`/v1/acl/clone/<id>`](#acl_clone): Creates a new token by cloning an existing token
* [`/v1/acl/list`](#acl_list): Lists all the active tokens

### <a name="acl_create"></a> /v1/acl/create

The `create` endpoint is used to make a new token. A token has a name,
a type, and a set of ACL rules.

The `Name` property is opaque to Consul. To aid human operators, it should
be a meaningful indicator of the ACL's purpose.

Type is either `client` or `management`. A management token is comparable
to a root user and has the ability to perform any action including
creating, modifying, and deleting ACLs.

By constrast, a client token can only perform actions as permitted by the
rules associated. Client tokens can never manage ACLs.  Given this limitation,
only a management token can be used to make requests to the `/v1/acl/create`
endpoint.

In any Consul cluster, only a single datacenter is authoritative for ACLs, so
all requests are automatically routed to that datacenter regardless
of the agent to which the request is made.

The create endpoint supports a JSON request body with the PUT. The request
body may take the form:

```javascript
{
  "Name": "my-app-token",
  "Type": "client",
  "Rules": ""
}
```

None of the fields are mandatory. In fact, no body needs to be PUT if the
defaults are to be used. The `Name` and `Rules` fields default to being
blank, and the `Type` defaults to "client".

The format of the `Rules` property is [documented here](/docs/internals/acl.html).

A successful response body will return the `ID` of the newly created ACL, like so:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

### <a name="acl_update"></a> /v1/acl/update

The update endpoint is used to modify the policy for a given ACL token. It
is very similar to the create endpoint; however, instead of generating a new
token ID, the `ID` field must be provided. As with [`/v1/acl/create`](#acl_create),
requests to this endpoint must be made with a management
token.

In any Consul cluster, only a single datacenter is authoritative for ACLs, so
all requests are automatically routed to that datacenter regardless
of the agent to which the request is made.

The update endpoint requires a JSON request body to the PUT. The request
body may look like:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
  "Name": "my-app-token-updated",
  "Type": "client",
  "Rules": "# New Rules",
}
```

Only the `ID` field is mandatory. The other fields provide defaults: the
`Name` and `Rules` fields default to being blank, and `Type` defaults to "client".
The format of `Rules` is [documented here](/docs/internals/acl.html).

### <a name="acl_destroy"></a> /v1/acl/destroy/\<id\>

The destroy endpoint must be hit with a PUT.  This endpoint destroys the ACL
token identified by the `id` portion of the path.

The request is automatically routed to the authoritative ACL datacenter.
Requests to this endpoint must be made with a management token.

### <a name="acl_info"></a> /v1/acl/info/\<id\>

The info endpoint must be hit with a GET.  This endpoint returns the ACL
token information identified by the `id` portion of the path.

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

If the ACL is not found, null is returned instead of a JSON list.

### <a name="acl_clone"></a> /v1/acl/clone/\<id\>

The clone endpoint must be hit with a PUT. It clones the ACL identified
by the `id` portion of the path and returns a new token `ID`. This allows
a token to serve as a template for others, making it simple to generate new
tokens without complex rule management.

The request is automatically routed to the authoritative ACL datacenter.
Requests to this endpoint must be made with a management token.

As with `create`, a successful response body will return the `ID` of the newly
created ACL, like so:

```javascript
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

### <a name="acl_list"></a> /v1/acl/list

The list endpoint must be hit with a GET. It lists all the active
ACL tokens. This is a privileged endpoint and requires a
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
