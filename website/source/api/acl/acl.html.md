---
layout: api
page_title: ACLs - HTTP API
sidebar_current: api-acl
description: |-
  The /acl endpoints manage the Consul's ACL system.
---

-> **1.4.0+:**  This API documentation is for Consul versions 1.4.0 and later. The documentation for the legacy ACL API is [here](/api/acl/legacy.html)

# ACL HTTP API

The `/acl` endpoints are used to manage ACL tokens and policies in Consul, [bootstrap the ACL system](#bootstrap-acls), [check ACL replication status](#check-acl-replication), and [translate rules](#translate-rules). There are additional pages for managing [tokens](/api/acl/tokens.html) and [policies](/api/acl/policies.html) with the `/acl` endpoints.

For more information about ACLs, please see the [ACL Guide](/docs/guides/acl.html).

## Bootstrap ACLs

This endpoint does a special one-time bootstrap of the ACL system, making the first
management token if the [`acl.tokens.master`](/docs/agent/options.html#acl_tokens_master)
configuration entry is not specified in the Consul server configuration and if the
cluster has not been bootstrapped previously. This is available in Consul 0.9.1 and later,
and requires all Consul servers to be upgraded in order to operate.

This provides a mechanism to bootstrap ACLs without having any secrets present in Consul's
configuration files.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/bootstrap`             | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `none`       |

### Sample Request

```text
$ curl \
    --request PUT \
    http://127.0.0.1:8500/v1/acl/bootstrap
```

### Sample Response

-> **Deprecated** - The `ID` field in the response is for legacy compatibility and is a copy of the `SecretID` field. New
applications should ignore the `ID` field as it may be removed in a future major Consul version.

```json
{
    "ID": "527347d3-9653-07dc-adc0-598b8f2b0f4d",
    "AccessorID": "b5b1a918-50bc-fc46-dec2-d481359da4e3",
    "SecretID": "527347d3-9653-07dc-adc0-598b8f2b0f4d",
    "Description": "Bootstrap Token (Global Management)",
    "Policies": [
        {
            "ID": "00000000-0000-0000-0000-000000000001",
            "Name": "global-management"
        }
    ],
    "Local": false,
    "CreateTime": "2018-10-24T10:34:20.843397-04:00",
    "Hash": "oyrov6+GFLjo/KZAfqgxF/X4J/3LX0435DOBy9V22I0=",
    "CreateIndex": 12,
    "ModifyIndex": 12
}
```

You can detect if something has interfered with the ACL bootstrapping process by
checking the response code. A 200 response means that the bootstrap was a success, and
a 403 means that the cluster has already been bootstrapped, at which point you should
consider the cluster in a potentially compromised state.

The returned token will have unrestricted privileges to manage all details of the system.
It can then be used to further configure the ACL system. Please see the
[ACL Guide](/docs/guides/acl.html) for more details.

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

## Translate Rules

-> **Deprecated** - This endpoint was introduced in Consul 1.4.0 for migration from the previous ACL system. It
will be removed in a future major Consul version when support for legacy ACLs is removed.

This endpoint translates the legacy rule syntax into the latest syntax. It is intended
to be used by operators managing Consul's ACLs and performing legacy token to new policy
migrations.

| Method | Path                        | Produces                   |
| ------ | --------------------------- | -------------------------- |
| `POST` | `/acl/rules/translate`      | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:read`   |

### Sample Payload

```text
agent "" {
   policy = "read"
}
```

### Sample Request

```text
$ curl -X POST -d @rules.hcl http://127.0.0.1:8500/v1/acl/rules/translate 
```

### Sample Response

```text
agent_prefix "" {
   policy = "read"
}
```

## Translate a Legacy Token's Rules

-> **Deprecated** - This endpoint was introduced in Consul 1.4.0 for migration from the previous ACL system.. It
will be removed in a future major Consul version when support for legacy ACLs is removed.

This endpoint translates the legacy rules embedded within a legacy ACL into the latest
syntax. It is intended to be used by operators managing Consul's ACLs and performing
legacy token to new policy migrations. Note that this API requires the auto-generated
Accessor ID of the legacy token. This ID can be retrieved using the
[`/v1/acl/token/self`](/api/acl/tokens.html#self) endpoint.

| Method | Path                                | Produces                   |
| ------ | ----------------------------------- | -------------------------- |
| `GET`  | `/acl/rules/translate/:accessor_id` | `text/plain`               |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes),
[agent caching](/api/index.html#agent-caching), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:read`   |

### Sample Request

```text
$ curl -X GET http://127.0.0.1:8500/v1/acl/rules/translate/4f48f7e6-9359-4890-8e67-6144a962b0a5
```

### Sample Response

```text
agent_prefix "" {
   policy = "read"
}
```
