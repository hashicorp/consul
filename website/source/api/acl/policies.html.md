---
layout: api
page_title: ACL Policies - HTTP API
sidebar_current: api-acl-policies
description: |-
  The /acl/policy endpoints manage Consul's ACL Policies.
---

-> **1.4.0+:**  The APIs are available in Consul versions 1.4.0 and later. The documentation for the legacy ACL API is [here](/api/acl/legacy.html)

# ACL Policy HTTP API

The `/acl/policy` endpoints [create](#create-a-policy), [read](#read-a-policy),
[update](#update-a-policy), [list](#list-policies) and
[delete](#delete-a-policy)  ACL policies in Consul.  

For more information on how to setup ACLs, please see
the [ACL Guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/production-acls).

## Create a Policy

This endpoint creates a new ACL policy.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/policy`                | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `Name` `(string: <required>)` - Specifies a name for the ACL policy. The name
  can contain alphanumeric characters, dashes `-`, and  underscores `_`.
  This name must be unique.

- `Description` `(string: "")` - Free form human readable description of the policy.

- `Rules` `(string: "")` - Specifies rules for the ACL policy. The format of the
  `Rules` property is detailed in the [ACL Rules documentation](/docs/acl/acl-rules.html).

- `Datacenters` `(array<string>)` - Specifies the datacenters the policy is valid within.
   When no datacenters are provided the policy is valid in all datacenters including
   those which do not yet exist but may in the future.

### Sample Payload

```json
{
  "Name": "node-read",
  "Description": "Grants read access to all node information",
  "Rules": "node_prefix \"\" { policy = \"read\"}",
  "Datacenters": ["dc1"]
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/policy
```

### Sample Response

```json
{
    "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
    "Name": "node-read",
    "Description": "Grants read access to all node information",
    "Rules": "node_prefix \"\" { policy = \"read\"}",
    "Datacenters": [
        "dc1"
    ],
    "Hash": "OtZUUKhInTLEqTPfNSSOYbRiSBKm3c4vI2p6MxZnGWc=",
    "CreateIndex": 14,
    "ModifyIndex": 14
}

```

## Read a Policy

This endpoint reads an ACL policy with the given ID.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/policy/:id`            | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

### Parameters

- `id` `(string: <required>)` - Specifies the UUID of the ACL policy to
  read. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/policy/e359bd81-baca-903e-7e64-1ccd9fdc78f5
```

### Sample Response

```json
{
    "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
    "Name": "node-read",
    "Description": "Grants read access to all node information",
    "Rules": "node_prefix \"\" { policy = \"read\"}",
    "Datacenters": [
        "dc1"
    ],
    "Hash": "OtZUUKhInTLEqTPfNSSOYbRiSBKm3c4vI2p6MxZnGWc=",
    "CreateIndex": 14,
    "ModifyIndex": 14
}
```

## Update a Policy

This endpoint updates an existing ACL policy.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/policy/:id`            | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `ID` `(string: <required>)` - Specifies the ID of the policy to update. This is
   required in the URL path but may also be specified in the JSON body. If specified
   in both places then they must match exactly.

- `Name` `(string: <required>)` - Specifies a name for the ACL policy. The name
   can only contain alphanumeric characters as well as `-` and `_` and must be
   unique.

- `Description` `(string: "")` - Free form human readable description of this policy.

- `Rules` `(string: "")` - Specifies rules for this ACL policy. The format of the
  `Rules` property is detailed in the [ACL Rules documentation](/docs/acl/acl-rules.html).

- `Datacenters` `(array<string>)` - Specifies the datacenters this policy is valid within.
   When no datacenters are provided the policy is valid in all datacenters including
   those which do not yet exist but may in the future.

### Sample Payload

```json
{
  "ID": "c01a1f82-44be-41b0-a686-685fb6e0f485",
  "Name": "register-app-service",
  "Description": "Grants write permissions necessary to register the 'app' service",
  "Rules": "service \"app\" { policy = \"write\"}",
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/policy/c01a1f82-44be-41b0-a686-685fb6e0f485
```

### Sample Response

```json
{
   "ID": "c01a1f82-44be-41b0-a686-685fb6e0f485",
   "Name": "register-app-service",
   "Description": "Grants write permissions necessary to register the 'app' service",
   "Rules": "service \"app\" { policy = \"write\"}",
   "Hash": "OtZUUKhInTLEqTPfNSSOYbRiSBKm3c4vI2p6MxZnGWc=",
   "CreateIndex": 14,
   "ModifyIndex": 28
}
```

## Delete a Policy

This endpoint deletes an ACL policy.

| Method   | Path                      | Produces                   |
| -------- | ------------------------- | -------------------------- |
| `DELETE` | `/acl/policy/:id`         | `application/json`         |

Even though the return type is application/json, the value is either true or
false indicating whether the delete succeeded.

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `id` `(string: <required>)` - Specifies the UUID of the ACL policy to
  delete. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X DELETE \
    http://127.0.0.1:8500/v1/acl/policy/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

### Sample Response
```json
true
```

## List Policies

This endpoint lists all the ACL policies.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/policies`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

## Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/policies
```

### Sample Response

-> **Note** - The policies rules are not included in the listing and must be
   retrieved by the [policy reading endpoint](#read-a-policy)

```json
[
    {
        "CreateIndex": 4,
        "Datacenters": null,
        "Description": "Builtin Policy that grants unlimited access",
        "Hash": "swIQt6up+s0cV4kePfJ2aRdKCLaQyykF4Hl1Nfdeumk=",
        "ID": "00000000-0000-0000-0000-000000000001",
        "ModifyIndex": 4,
        "Name": "global-management"
    },
    {
        "CreateIndex": 14,
        "Datacenters": [
            "dc1"
        ],
        "Description": "Grants read access to all node information",
        "Hash": "OtZUUKhInTLEqTPfNSSOYbRiSBKm3c4vI2p6MxZnGWc=",
        "ID": "e359bd81-baca-903e-7e64-1ccd9fdc78f5",
        "ModifyIndex": 14,
        "Name": "node-read"
    }
]

```
