---
layout: api
page_title: Legacy ACLs - HTTP API
sidebar_current: api-acl-tokens-legacy
description: |-
  The /acl endpoints create, update, destroy, and query Legacy ACL tokens in Consul.
---

-> **Consul 1.4.0 deprecates the legacy ACL system completely.** It's _strongly_
recommended you do not build anything using the legacy system and consider using
the new ACL [Token](/api/acl/tokens.html) and [Policy](/api/acl/policies.html) APIs instead.

# ACL HTTP API

The `/acl` endpoints create, update, destroy, and query ACL tokens in Consul.

For more information about ACLs, please see the [ACL Guide](https://learn.hashicorp.com/consul/security-networking/production-acls).

## Create ACL Token

This endpoint makes a new ACL token.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/create`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `management` |

### Parameters

- `ID` `(string: "")` - Specifies the ID of the ACL. If not provided, a UUID is
  generated.

- `Name` `(string: "")` - Specifies a human-friendly name for the ACL token.

- `Type` `(string: "client")` - Specifies the type of ACL token. Valid values
  are: `client` and `management`.

- `Rules` `(string: "")` - Specifies rules for this ACL token. The format of the
  `Rules` property is detailed in the [ACL Rule documentation](/docs/acl/acl-rules.html).

### Sample Payload

```json
{
  "Name": "my-app-token",
  "Type": "client",
  "Rules": ""
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/create
```

### Sample Response

```json
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

## Update ACL Token

This endpoint is used to modify the policy for a given ACL token. Instead of
generating a new token ID, the `ID` field must be provided.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/update`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `management` |

### Parameters

The parameters are the same as the _create_ endpoint, except the `ID` field is
required.

### Sample Payload

```json
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
  "Name": "my-app-token-updated",
  "Type": "client",
  "Rules": "# New Rules",
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/update
```

### Sample Response
```json
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```


## Delete ACL Token

This endpoint deletes an ACL token with the given ID.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/destroy/:uuid`         | `application/json`         |

Even though the return type is application/json, the value is either true or
false, indicating whether the delete succeeded.

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `management` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the ACL token to
  destroy. This is required and is specified as part of the URL path.

### Sample Request

```text
$ curl \
    --request PUT \
    http://127.0.0.1:8500/v1/acl/destroy/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

### Sample Response
```json
true
```

## Read ACL Token

This endpoint reads an ACL token with the given ID.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/info/:uuid`            | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `none`       |

Note: No ACL is required because the ACL is specified in the URL path.

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the ACL token to
  read. This is required and is specified as part of the URL path.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/acl/info/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

### Sample Response

```json
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

## Clone ACL Token

This endpoint clones an ACL and returns a new token `ID`. This allows a token to
serve as a template for others, making it simple to generate new tokens without
complex rule management.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/clone/:uuid`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `management` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the ACL token to
  be cloned. This is required and is specified as part of the URL path.

### Sample Request

```text
$ curl \
    --request PUT \
    http://127.0.0.1:8500/v1/acl/clone/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

### Sample Response

```json
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

## List ACLs

This endpoint lists all the active ACL tokens.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/list`                  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `management` |

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/acl/list
```

### Sample Response

```json
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


## Check ACL Replication

The check ACL replication endpoint has not changed between the legacy system and the new system. Review the [latest documentation](/api/acl/acl.html#check-acl-replication) to learn more about this endpoint. 

