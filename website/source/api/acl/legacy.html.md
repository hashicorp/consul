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

The `/acl` endpoints create, update, destroy, and query ACL tokens in Consul. For more information about ACLs, please see the [ACL Guide](/docs/guides/acl-legacy.html).

## Create ACL Token

This endpoint makes a new ACL token.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/create`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

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
  `Rules` property is documented in the [ACL Guide](/docs/guides/acl-legacy.html).

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
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

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
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

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
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

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
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

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
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

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

This endpoint returns the status of the ACL replication processes in the
datacenter. This is intended to be used by operators or by automation checking 
to discover the health of ACL replication.

Please see the [ACL Guide](/docs/guides/acl.html#replication) replication
section for more details.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/replication`           | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `consistent`      | `none`        | `none`       |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

### Sample Request

```text
$ curl \
    --request GET \
    http://127.0.0.1:8500/v1/acl/replication
```

### Sample Response

```json
{
  "Enabled": true,
  "Running": true,
  "SourceDatacenter": "dc1",
  "ReplicationType" : "tokens",
  "ReplicatedIndex": 1976,
  "ReplicatedTokenIndex": 2018,
  "LastSuccess": "2018-11-03T06:28:58Z",
  "LastError": "2016-11-03T06:28:28Z"
}
```

- `Enabled` - Reports whether ACL replication is enabled for the datacenter.

- `Running` - Reports whether the ACL replication process is running. The process
  may take approximately 60 seconds to begin running after a leader election
  occurs.

- `SourceDatacenter` - The authoritative ACL datacenter that ACLs are being
  replicated from and will match the
  [`primary_datacenter`](/docs/agent/options.html#primary_datacenter) configuration.

- `ReplicationType` - The type of replication that is currently in use.

   - `legacy` - ACL replication is in legacy mode and is replicating legacy ACL tokens.

   - `policies` - ACL replication is only replicating policies as token replication
     is disabled.

   - `tokens` - ACL replication is replicating both policies and tokens.

- `ReplicatedIndex` - The last index that was successfully replicated. Which data
  the replicated index refers to depends on the replication type. For `legacy`
  replication this can be compared with the value of the `X-Consul-Index` header
  returned by the [`/v1/acl/list`](/api/acl/legacy.html#acl_list) endpoint to
  determine if the replication process has gotten all available ACLs. When in either
  `tokens` or `policies` mode, this index can be compared with the value of the
  `X-Consul-Index` header returned by the [`/v1/acl/polcies`](/api/acl/policies.html#list-policies)
  endpoint to determine if the policy replication process has gotten all available
  ACL policies. Note that ACL replication is rate limited so the indexes may lag behind
  the primary datacenter.

- `ReplicatedTokenIndex` - The last token index that was successfully replicated.
   This index can be compared with the value of the `X-Consul-Index` header returned
   by the [`/v1/acl/tokens`](/api/acl/tokens.html#list) endpoint to determine
   if the replication process has gotten all available ACL tokens. Note that ACL
   replication is rate limited so the indexes may lag behind the primary
   datacenter.

- `LastSuccess` - The UTC time of the last successful sync operation. Since ACL
  replication is done with a blocking query, this may not update for up to 5
  minutes if there have been no ACL changes to replicate. A zero value of
  "0001-01-01T00:00:00Z" will be present if no sync has been successful.

- `LastError` - The UTC time of the last error encountered during a sync
  operation. If this time is later than `LastSuccess`, you can assume the
  replication process is not in a good state. A zero value of
  "0001-01-01T00:00:00Z" will be present if no sync has resulted in an error.