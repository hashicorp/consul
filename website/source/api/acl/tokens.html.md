---
layout: api
page_title: ACL Tokens - HTTP API
sidebar_current: api-acl-tokens
description: |-
  The /acl/token endpoints manage Consul's ACL Tokens.
---

-> **1.4.0+:**  The APIs are available in Consul versions 1.4.0 and later. The documentation for the legacy ACL API is [here](/api/acl/legacy.html)

# ACL Token HTTP API

The `/acl/token` endpoints [create](#create-a-token), [read](#read-a-token),
[update](#update-a-token), [list](#list-tokens), [clone](#clone-a-token) and [delete](#delete-a-token)  ACL policies in Consul.

For more information on how to setup ACLs, please see
the [ACL Guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/production-acls).

## Create a Token

This endpoint creates a new ACL token.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/token`                 | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `AccessorID` `(string: "")` - Specifies a UUID to use as the token's Accessor ID.
   If not specified a UUID will be generated for this field. Added in v1.5.0.

- `SecretID` `(string: "")` - Specifies a UUID to use as the token's Secret ID.
   If not specified a UUID will be generated for this field. Added in v1.5.0.
   **Note**: The SecretID is used to authorize operations against Consul and should
   be generated from an appropriate cryptographic source.

- `Description` `(string: "")` - Free form human readable description of the token.

- `Policies` `(array<PolicyLink>)` - The list of policies that should be
  applied to the token. A PolicyLink is an object with an "ID" and/or "Name"
  field to specify a policy. With the PolicyLink, tokens can be linked to
  policies either by the policy name or by the policy ID. When policies are
  linked by name they will be internally resolved to the policy ID. With
  linking tokens internally by IDs, Consul enables policy renaming without
  breaking tokens.

- `Roles` `(array<RoleLink>)` - The list of roles that should be applied to the
  token. A RoleLink is an object with an "ID" and/or "Name" field to specify a
  role. With the RoleLink, tokens can be linked to roles either by the role
  name or by the role ID. When roles are linked by name they will be internally
  resolved to the role ID. With linking tokens internally by IDs, Consul
  enables role renaming without breaking tokens. Added in Consul 1.5.0.

- `ServiceIdentities` `(array<ServiceIdentity>)` - The list of [service
  identities](/docs/acl/acl-system.html#acl-service-identities) that should be
  applied to the token.  Added in Consul 1.5.0.

  - `ServiceName` `(string: <required>)` - The name of the service. The name
    must be no longer than 256 characters, must start and end with a lowercase
    alphanumeric character, and can only contain lowercase alphanumeric
    characters as well as `-` and `_`.

  - `Datacenters` `(array<string>)` - Specifies the datacenters the effective
    policy is valid within. When no datacenters are provided the effective
    policy is valid in all datacenters including those which do not yet exist
    but may in the future.

- `Local` `(bool: false)` - If true, indicates that the token should not be
  replicated globally and instead be local to the current datacenter.

- `ExpirationTime` `(time: "")`- If set this represents the point after which a
  token should be considered revoked and is eligible for destruction. The
  default unset value represents NO expiration. This value must be between 1
  minute and 24 hours in the future. Added in Consul 1.5.0.

- `ExpirationTTL` `(duration: 0s)` - This is a convenience field and if set
  will initialize the `ExpirationTime` field to a value of `CreateTime +
  ExpirationTTL`. This field is not persisted beyond its initial use. Can be
  specified in the form of `"60s"` or `"5m"` (i.e., 60 seconds or 5 minutes,
  respectively). This value must be no smaller than 1 minute and no longer than
  24 hours. Added in Consul 1.5.0.

### Sample Payload

```json
{
   "Description": "Agent token for 'node1'",
   "Policies": [
      {
         "ID": "165d4317-e379-f732-ce70-86278c4558f7"
      },
      {
         "Name": "node-read"
      }
   ],
   "Local": false
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/token
```

### Sample Response

```json
{
    "AccessorID": "6a1253d2-1785-24fd-91c2-f8e78c745511",
    "SecretID": "45a3bd52-07c7-47a4-52fd-0745e0cfe967",
    "Description": "Agent token for 'node1'",
    "Policies": [
        {
            "ID": "165d4317-e379-f732-ce70-86278c4558f7",
            "Name": "node1-write"
        },
        {
            "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
            "Name": "node-read"
        }
    ],
    "Local": false,
    "CreateTime": "2018-10-24T12:25:06.921933-04:00",
    "Hash": "UuiRkOQPRCvoRZHRtUxxbrmwZ5crYrOdZ0Z1FTFbTbA=",
    "CreateIndex": 59,
    "ModifyIndex": 59
}
```


## Read a Token

This endpoint reads an ACL token with the given Accessor ID.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/token/:AccessorID`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

### Parameters

- `AccessorID` `(string: <required>)` - Specifies the accessor ID of the ACL token to
  read. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/token/6a1253d2-1785-24fd-91c2-f8e78c745511
```

### Sample Response

-> **Note** If the token used for accessing the API has `acl:write` permissions,
then the `SecretID` will contain the tokens real value. Only when accessed with
a token with only `acl:read` permissions will the `SecretID` be redacted. This
is to prevent privilege escalation whereby having `acl:read` privileges allows
for reading other secrets which given even more permissions.

```json
{
    "AccessorID": "6a1253d2-1785-24fd-91c2-f8e78c745511",
    "SecretID": "<hidden>",
    "Description": "Agent token for 'node1'",
    "Policies": [
        {
            "ID": "165d4317-e379-f732-ce70-86278c4558f7",
            "Name": "node1-write"
        },
        {
            "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
            "Name": "node-read"
        }
    ],
    "Local": false,
    "CreateTime": "2018-10-24T12:25:06.921933-04:00",
    "Hash": "UuiRkOQPRCvoRZHRtUxxbrmwZ5crYrOdZ0Z1FTFbTbA=",
    "CreateIndex": 59,
    "ModifyIndex": 59
}
```

## Read Self Token

This endpoint returns the ACL token details that matches the secret ID
specified with the `X-Consul-Token` header or the `token` query parameter.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/token/self`            | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `none`       |

-> **Note** - This endpoint requires no specific privileges as it is just
retrieving the data for a token that you must already possess its secret.

### Sample Request

```sh
$ curl -H "X-Consul-Token: 6a1253d2-1785-24fd-91c2-f8e78c745511" \
   http://127.0.0.1:8500/v1/acl/token/self
```

### Sample Response

```json
{
    "AccessorID": "6a1253d2-1785-24fd-91c2-f8e78c745511",
    "SecretID": "45a3bd52-07c7-47a4-52fd-0745e0cfe967",
    "Description": "Agent token for 'node1'",
    "Policies": [
        {
            "ID": "165d4317-e379-f732-ce70-86278c4558f7",
            "Name": "node1-write"
        },
        {
            "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
            "Name": "node-read"
        }
    ],
    "Local": false,
    "CreateTime": "2018-10-24T12:25:06.921933-04:00",
    "Hash": "UuiRkOQPRCvoRZHRtUxxbrmwZ5crYrOdZ0Z1FTFbTbA=",
    "CreateIndex": 59,
    "ModifyIndex": 59
}
```


## Update a Token

This endpoint updates an existing ACL token.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/token/:AccessorID`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `AccessorID` `(string: "")` - Specifies the accessor ID of the token being updated. This is
   required in the URL path but may also be specified in the JSON body. If specified
   in both places then they must match exactly. This field is immutable. If not present in
   the body and only in the URL then it will be filled in by Consul.

- `SecretID` `(string: "")` - Specifies the secret ID of the token being updated. This field is
   immutable so if present in the body then it must match the existing value. If not present
   then the value will be filled in by Consul.

- `Description` `(string: "")` - Free form human readable description of this token.

- `Policies` `(array<PolicyLink>)` - This is the list of policies that should
  be applied to this token. A PolicyLink is an object with an "ID" and/or
  "Name" field to specify a policy. With the PolicyLink tokens can be linked to policies
  either by the policy name or by the policy ID. When policies are linked by
  name they will internally be resolved to the policy ID. With linking tokens
  internally by IDs, Consul enables policy renaming without breaking tokens.

- `Roles` `(array<RoleLink>)` - The list of roles that should be applied to the
  token. A RoleLink is an object with an "ID" and/or "Name" field to specify a
  role. With the RoleLink, tokens can be linked to roles either by the role
  name or by the role ID. When roles are linked by name they will be internally
  resolved to the role ID. With linking tokens internally by IDs, Consul
  enables role renaming without breaking tokens.

- `ServiceIdentities` `(array<ServiceIdentity>)` - The list of [service
  identities](/docs/acl/acl-system.html#acl-service-identities) that should be
  applied to the token. Added in Consul 1.5.0.

  - `ServiceName` `(string: <required>)` - The name of the service. The name
    must be no longer than 256 characters, must start and end with a lowercase
    alphanumeric character, and can only contain lowercase alphanumeric
    characters as well as `-` and `_`.

  - `Datacenters` `(array<string>)` - Specifies the datacenters the effective
    policy is valid within. When no datacenters are provided the effective
    policy is valid in all datacenters including those which do not yet exist
    but may in the future.

- `Local` `(bool: false)` - If true, indicates that this token should not be
  replicated globally and instead be local to the current datacenter. This
  value must match the existing value or the request will return an error.

- `AuthMethod` `(string: "")` - Specifies the name of the auth method that
  created this token. This field is immutable so if present in the body then it
  must match the existing value. If not present then the value will be filled
  in by Consul.

- `ExpirationTime` `(time: "")` - Specifies the expiration time for the token
  being updated. This field is immutable so if present in the body then it must
  match the existing value. If not present then the value will be filled in by
  Consul.


### Sample Payload

```json
{
   "Description": "Agent token for 'node1'",
   "Policies": [
      {
         "ID": "165d4317-e379-f732-ce70-86278c4558f7"
      },
      {
         "Name": "node-read"
      },
      {
         "Name": "service-read"
      }
   ],
   "Local": false
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/token/6a1253d2-1785-24fd-91c2-f8e78c745511
```

### Sample Response

```json
{
    "AccessorID": "6a1253d2-1785-24fd-91c2-f8e78c745511",
    "SecretID": "45a3bd52-07c7-47a4-52fd-0745e0cfe967",
    "Description": "Agent token for 'node1'",
    "Policies": [
        {
            "ID": "165d4317-e379-f732-ce70-86278c4558f7",
            "Name": "node1-write"
        },
        {
            "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
            "Name": "node-read"
        },
        {
            "ID": "93d2226b-2046-4db1-993b-c0581b5d2391",
            "Name": "service-read"
        }
    ],
    "Local": false,
    "CreateTime": "2018-10-24T12:25:06.921933-04:00",
    "Hash": "UuiRkOQPRCvoRZHRtUxxbrmwZ5crYrOdZ0Z1FTFbTbA=",
    "CreateIndex": 59,
    "ModifyIndex": 100
}
```

## Clone a Token

This endpoint clones an existing ACL token.

| Method | Path                           | Produces                   |
| ------ | ------------------------------ | -------------------------- |
| `PUT`  | `/acl/token/:AccessorID/clone` | `application/json`        |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `AccessorID` `(string: <required>)` - The accessor ID of the token to clone. This is required
   in the URL path

- `Description` `(string: "")` - Free form human readable description for the cloned token.

### Sample Payload

```json
{
   "Description": "Clone of Agent token for 'node1'",
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/token/6a1253d2-1785-24fd-91c2-f8e78c745511/clone
```

### Sample Response

```json
{
    "AccessorID": "773efe2a-1f6f-451f-878c-71be10712bae",
    "SecretID": "8b1247ef-d172-4f99-b050-4dbe5d3df0cb",
    "Description": "Clone of Agent token for 'node1'",
    "Policies": [
        {
            "ID": "165d4317-e379-f732-ce70-86278c4558f7",
            "Name": "node1-write"
        },
        {
            "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
            "Name": "node-read"
        },
        {
            "ID": "93d2226b-2046-4db1-993b-c0581b5d2391",
            "Name": "service-read"
        }
    ],
    "Local": false,
    "CreateTime": "2018-10-24T12:25:06.921933-04:00",
    "Hash": "UuiRkOQPRCvoRZHRtUxxbrmwZ5crYrOdZ0Z1FTFbTbA=",
    "CreateIndex": 128,
    "ModifyIndex": 128
}
```

## Delete a Token

This endpoint deletes an ACL token.

| Method   | Path                      | Produces                   |
| -------- | ------------------------- | -------------------------- |
| `DELETE` | `/acl/token/:AccessorID`  | `application/json`         |

Even though the return type is application/json, the value is either true or
false, indicating whether the delete succeeded.

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `AccessorID` `(string: <required>)` - Specifies the accessor ID of the ACL policy to
  delete. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X DELETE \
    http://127.0.0.1:8500/v1/acl/token/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

### Sample Response
```json
true
```

## List Tokens

This endpoint lists all the ACL tokens.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/tokens`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

## Parameters

- `policy` `(string: "")` - Filters the token list to those tokens that are
  linked with the specific policy ID.

- `role` `(string: "")` - Filters the token list to those tokens that are
  linked with the specific role ID.

- `authmethod` `(string: "")` - Filters the token list to those tokens that are
  linked with the specific named auth method.

## Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/tokens
```

### Sample Response

-> **Note** - The token secret IDs are not included in the listing and must be
   retrieved by the [token reading endpoint](#read-a-token)

```json
[
    {
        "AccessorID": "6a1253d2-1785-24fd-91c2-f8e78c745511",
        "Description": "Agent token for 'my-agent'",
        "Policies": [
            {
                "ID": "165d4317-e379-f732-ce70-86278c4558f7",
                "Name": "node1-write"
            },
            {
                "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
                "Name": "node-read"
            }
        ],
        "Local": false,
        "CreateTime": "2018-10-24T12:25:06.921933-04:00",
        "Hash": "UuiRkOQPRCvoRZHRtUxxbrmwZ5crYrOdZ0Z1FTFbTbA=",
        "CreateIndex": 59,
        "ModifyIndex": 59
    },
    {
        "AccessorID": "00000000-0000-0000-0000-000000000002",
        "Description": "Anonymous Token",
        "Policies": null,
        "Local": false,
        "CreateTime": "0001-01-01T00:00:00Z",
        "Hash": "RNVFSWnfd5DUOuB8vplp+imivlIna3fKQVnkUHh21cA=",
        "CreateIndex": 5,
        "ModifyIndex": 5
    },
    {
        "AccessorID": "3328f9a6-433c-02d0-6649-7d07268dfec7",
        "Description": "Bootstrap Token (Global Management)",
        "Policies": [
            {
                "ID": "00000000-0000-0000-0000-000000000001",
                "Name": "global-management"
            }
        ],
        "Local": false,
        "CreateTime": "2018-10-24T11:42:02.6427-04:00",
        "Hash": "oyrov6+GFLjo/KZAfqgxF/X4J/3LX0435DOBy9V22I0=",
        "CreateIndex": 12,
        "ModifyIndex": 12
    }
]
```
