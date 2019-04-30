---
layout: api
page_title: ACL Roles - HTTP API
sidebar_current: api-acl-roles
description: |-
  The /acl/role endpoints manage Consul's ACL Roles.
---

-> **1.5.0+:**  The role APIs are available in Consul versions 1.5.0 and newer.

# ACL Role HTTP API

The `/acl/role` endpoints [create](#create-a-role), [read](#read-a-role),
[update](#update-a-role), [list](#list-roles) and [delete](#delete-a-role)  ACL roles in Consul.

For more information on how to setup ACLs, please see
the [ACL Guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/production-acls).

## Create a Role

This endpoint creates a new ACL role.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/role`                  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `Name` `(string: <required>)` - Specifies a name for the ACL role. The name
  can contain alphanumeric characters, dashes `-`, and  underscores `_`.
  This name must be unique.
   
- `Description` `(string: "")` - Free form human readable description of the role.

- `Policies` `(array<PolicyLink>)` - The list of policies that should be
  applied to the role. A PolicyLink is an object with an "ID" and/or "Name"
  field to specify a policy. With the PolicyLink, roles can be linked to
  policies either by the policy name or by the policy ID. When policies are
  linked by name they will be internally resolved to the policy ID. With
  linking roles internally by IDs, Consul enables policy renaming without
  breaking tokens.

- `ServiceIdentities` `(array<ServiceIdentity>)` - The list of [service
  identities](/docs/acl/acl-system.html#acl-service-identities) that should be
  applied to the role.  Added in Consul 1.5.0.

  - `ServiceName` `(string: <required>)` - The name of the service. The name
    must be no longer than 256 characters, must start and end with a lowercase
    alphanumeric character, and can only contain lowercase alphanumeric
    characters as well as `-` and `_`.

  - `Datacenters` `(array<string>)` - Specifies the datacenters the effective
    policy is valid within. When no datacenters are provided the effective
    policy is valid in all datacenters including those which do not yet exist
    but may in the future.

### Sample Payload

```json
{
    "Name": "example-role",
    "Description": "Showcases all input parameters",
    "Policies": [
        {
            "ID": "783beef3-783f-f41f-7422-7087dc272765"
        },
        {
            "Name": "node-read"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "web"
        },
        {
            "ServiceName": "db",
            "Datacenters": [
                "dc1"
            ]
        }
    ]
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/role
```

### Sample Response

```json
{
    "ID": "aa770e5b-8b0b-7fcf-e5a1-8535fcc388b4",
    "Name": "example-role",
    "Description": "Showcases all input parameters",
    "Policies": [
        {
            "ID": "783beef3-783f-f41f-7422-7087dc272765",
            "Name": "node-read"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "web"
        },
        {
            "ServiceName": "db",
            "Datacenters": [
                "dc1"
            ]
        }
    ],
    "Hash": "mBWMIeX9zyUTdDMq8vWB0iYod+mKBArJoAhj6oPz3BI=",
    "CreateIndex": 57,
    "ModifyIndex": 57
}
```

## Read a Role

This endpoint reads an ACL role with the given ID. If no role exists with the
given ID, a 404 is returned instead of a 200 response.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/role/:id`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

### Parameters

- `id` `(string: <required>)` - Specifies the UUID of the ACL role to
   read. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/role/aa770e5b-8b0b-7fcf-e5a1-8535fcc388b4
```

### Sample Response

```json
{
    "ID": "aa770e5b-8b0b-7fcf-e5a1-8535fcc388b4",
    "Name": "example-role",
    "Description": "Showcases all input parameters",
    "Policies": [
        {
            "ID": "783beef3-783f-f41f-7422-7087dc272765",
            "Name": "node-read"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "web"
        },
        {
            "ServiceName": "db",
            "Datacenters": [
                "dc1"
            ]
        }
    ],
    "Hash": "mBWMIeX9zyUTdDMq8vWB0iYod+mKBArJoAhj6oPz3BI=",
    "CreateIndex": 57,
    "ModifyIndex": 57
}
```

## Read a Role by Name

This endpoint reads an ACL role with the given name. If no role exists with the
given name, a 404 is returned instead of a 200 response.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/role/name/:name`       | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

### Parameters

- `name` `(string: <required>)` - Specifies the Name of the ACL role to
   read. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/role/name/example-role
```

### Sample Response

```json
{
    "ID": "aa770e5b-8b0b-7fcf-e5a1-8535fcc388b4",
    "Name": "example-role",
    "Description": "Showcases all input parameters",
    "Policies": [
        {
            "ID": "783beef3-783f-f41f-7422-7087dc272765",
            "Name": "node-read"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "web"
        },
        {
            "ServiceName": "db",
            "Datacenters": [
                "dc1"
            ]
        }
    ],
    "Hash": "mBWMIeX9zyUTdDMq8vWB0iYod+mKBArJoAhj6oPz3BI=",
    "CreateIndex": 57,
    "ModifyIndex": 57
}
```

## Update a Role

This endpoint updates an existing ACL role.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/role/:id`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `ID` `(string: <required>)` - Specifies the ID of the role to update. This is
   required in the URL path but may also be specified in the JSON body. If specified
   in both places then they must match exactly.

- `Name` `(string: <required>)` - Specifies a name for the ACL role. The name
  can only contain alphanumeric characters as well as `-` and `_` and must be
  unique. 
   
- `Description` `(string: "")` - Free form human readable description of the role.

- `Policies` `(array<PolicyLink>)` - The list of policies that should be
  applied to the role. A PolicyLink is an object with an "ID" and/or "Name"
  field to specify a policy. With the PolicyLink, roles can be linked to
  policies either by the policy name or by the policy ID. When policies are
  linked by name they will be internally resolved to the policy ID. With
  linking roles internally by IDs, Consul enables policy renaming without
  breaking tokens.

- `ServiceIdentities` `(array<ServiceIdentity>)` - The list of [service
  identities](/docs/acl/acl-system.html#acl-service-identities) that should be
  applied to the role.  Added in Consul 1.5.0.

### Sample Payload

```json
{
    "ID": "8bec74a4-5ced-45ed-9c9d-bca6153490bb",
    "Name": "example-two",
    "Policies": [
        {
            "Name": "node-read"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "db"
        }
    ]
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/role/8bec74a4-5ced-45ed-9c9d-bca6153490bb
```

### Sample Response

```json
{
    "ID": "8bec74a4-5ced-45ed-9c9d-bca6153490bb",
    "Name": "example-two",
    "Policies": [
        {
            "ID": "783beef3-783f-f41f-7422-7087dc272765",
            "Name": "node-read"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "db"
        }
    ],
    "Hash": "OtZUUKhInTLEqTPfNSSOYbRiSBKm3c4vI2p6MxZnGWc=",
    "CreateIndex": 14,
    "ModifyIndex": 28
}
```

## Delete a Role

This endpoint deletes an ACL role.

| Method   | Path                      | Produces                   |
| -------- | ------------------------- | -------------------------- |
| `DELETE` | `/acl/role/:id`           | `application/json`         |

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

- `id` `(string: <required>)` - Specifies the UUID of the ACL role to
  delete. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X DELETE \
    http://127.0.0.1:8500/v1/acl/role/8f246b77-f3e1-ff88-5b48-8ec93abf3e05
```

### Sample Response
```json
true
```

## List Roles

This endpoint lists all the ACL roles.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/roles`                 | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

## Parameters

- `policy` `(string: "")` - Filters the role list to those roles that are
  linked with the specific policy ID.

## Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/roles
```

### Sample Response

```json
[
    {
        "ID": "5e52a099-4c90-c067-5478-980f06be9af5",
        "Name": "node-read",
        "Description": "",
        "Policies": [
            {
                "ID": "783beef3-783f-f41f-7422-7087dc272765",
                "Name": "node-read"
            }
        ],
        "Hash": "K6AbfofgiZ1BEaKORBloZf7WPdg45J/PipHxQiBlK1U=",
        "CreateIndex": 50,
        "ModifyIndex": 50
    },
    {
        "ID": "aa770e5b-8b0b-7fcf-e5a1-8535fcc388b4",
        "Name": "example-role",
        "Description": "Showcases all input parameters",
        "Policies": [
            {
                "ID": "783beef3-783f-f41f-7422-7087dc272765",
                "Name": "node-read"
            }
        ],
        "ServiceIdentities": [
            {
                "ServiceName": "web"
            },
            {
                "ServiceName": "db",
                "Datacenters": [
                    "dc1"
                ]
            }
        ],
        "Hash": "mBWMIeX9zyUTdDMq8vWB0iYod+mKBArJoAhj6oPz3BI=",
        "CreateIndex": 57,
        "ModifyIndex": 57
    }
]
```
