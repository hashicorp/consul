---
layout: api
page_title: Namespace - HTTP API
sidebar_current: api-namespaces
description: |-
   The /namespace endpoints allow for managing Consul Enterprise Namespaces.
---

# Namespace - HTTP API

~> **Enterprise Only!** These API endpoints and functionality only exists in
Consul Enterprise. This is not present in the open source version of Consul.

The functionality described here is available only in
[Consul Enterprise](https://www.hashicorp.com/products/consul/) version 1.7.0 and later.

## Create a Namespace

This endpoint creates a new ACL token.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/namespace`                 | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required      |
| ---------------- | ----------------- | ------------- | ----------------- |
| `NO`             | `none`            | `none`        | `operator:write`  |

### Parameters

- `Name` `(string: <required>)` - The namespaces name. This must be a valid
  DNS hostname label.

- `Description` `(string: "")` - Free form namespaces description.

- `ACLs` `(object: <optional>)` - ACL configurations for this namespace. Rules from
  default policies and roles will be used only when there are no rules from directly linked
  policies, roles and service identities that are for the target resource and segment.
  Therefore if a directly linked policy grants read access to some resource and a 
  default policy grants write access, the effective access for the token will be read 
  due to the default policies not being checked. When there is no rule concerning
  the resource in either the directly linked policies, roles and service identities
  nor in those linked by the defaults, then the agents default policy configuration
  will be used for making the enforcement decision.

  - `PolicyDefaults` `(array<ACLLink>)` - This is the list of default policies
    that should be applied to all tokens created in this namespace. The ACLLink
    struct is an object with an "ID" and/or "Name" field to identify a policy. 
    When a name is used instead of an ID, Consul will resolve the name to an ID
    and store that internally.
    
   - `RoleDefaults` `(array<ACLLink>)` - This is the list of default roles
    that should be applied to all tokens created in this namespace. The ACLLink
    struct is an object with an "ID" and/or "Name" field to identify a policy. 
    When a name is used instead of an ID, Consul will resolve the name to an ID
    and store that internally.

- `Meta` `(map<string|string>: <optional>)` - Specifies arbitrary KV metadata
  to associate with the namespace.

### Sample Payload

```json
{
   "Name": "team-1",
   "Description": "Namespace for Team 1",
   "ACLs": {
      "PolicyDefaults": [
         {
            "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1"
         },
         {
            "Name": "node-read"
         }
      ],
      "RoleDefaults": [
         {
            "ID": "69748856-ae69-d620-3ec4-07844b3c6be7"
         },
         {
            "Name": "ns-team-2-read"
         }
      ]
   },
   "Meta": {
      "foo": "bar"
   }
}
```

### Sample Request

```sh
$ curl -X PUT \
   -H "X-Consul-Token: 5cdcae6c-0cce-4210-86fe-5dff3b984a6e" \
   --data @payload.json \
   http://127.0.0.1:8500/v1/namespace
```

### SampleResponse

```json
{
    "Name": "team-1",
    "Description": "Namespace for Team 1",
    "ACLs": {
        "PolicyDefaults": [
            {
                "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1",
                "Name": "service-read"
            },
            {
                "ID": "af937401-9950-fcae-8396-610ce047649a",
                "Name": "node-read"
            }
        ],
        "RoleDefaults": [
            {
                "ID": "69748856-ae69-d620-3ec4-07844b3c6be7",
                "Name": "service-discovery"
            },
            {
                "ID": "ae4b3542-d824-eb5f-7799-3fd657847e4e",
                "Name": "ns-team-2-read"
            }
        ]
    },
    "Meta": {
        "foo": "bar"
    },
    "CreateIndex": 55,
    "ModifyIndex": 55
}
```

## Read a Namespace

This endpoint reads a Namespace with the given name.

| Method | Path                   | Produces                   |
| ------ | ---------------------- | -------------------------- |
| `GET`  | `/namespace/:name`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `operator:read` or `namespace:*:read`<sup>1<sup>   |

<sup>1</sup> Access can be granted to list the Namespace if the token used when making 
the request has been granted any access in the namespace (read, list or write).

### Parameters

- `name` `(string: <required>)` - Specifies the namespace to read. This
  is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -H "X-Consul-Token: b23b3cad-5ea1-4413-919e-c76884b9ad60" \
   http://127.0.0.1:8500/v1/namespace/team-1
```

### SampleResponse

```json
{
    "Name": "team-1",
    "Description": "Namespace for Team 1",
    "ACLs": {
        "PolicyDefaults": [
            {
                "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1",
                "Name": "service-read"
            },
            {
                "ID": "af937401-9950-fcae-8396-610ce047649a",
                "Name": "node-read"
            }
        ],
        "RoleDefaults": [
            {
                "ID": "69748856-ae69-d620-3ec4-07844b3c6be7",
                "Name": "service-discovery"
            },
            {
                "ID": "ae4b3542-d824-eb5f-7799-3fd657847e4e",
                "Name": "ns-team-2-read"
            }
        ]
    },
    "Meta": {
        "foo": "bar"
    },
    "CreateIndex": 55,
    "ModifyIndex": 55
}
```

## Update a Namespace

This endpoint updates a new ACL token.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/namespace/:name`                 | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required      |
| ---------------- | ----------------- | ------------- | ----------------- |
| `NO`             | `none`            | `none`        | `operator:write`  |

### Parameters

- `Name` `(string: <optional>)` - The namespaces name. This must be a valid
  DNS hostname label. If present in the payload it must match what was given
  in the URL path.

- `Description` `(string: "")` - Free form namespaces description.

- `ACLs` `(object: <optional>)` - ACL configurations for this Namespace. Rules from
  default policies and roles will be used only when there are no rules from directly linked
  policies, roles and service identities that are for the target resource and segment.
  Therefore if a directly linked policy grants read access to some resource and a 
  default policy grants write access, the effective access for the token will be read 
  due to the default policies not being checked. When there is no rule concerning
  the resource in either the directly linked policies, roles and service identities
  nor in those linked by the defaults, then the agents default policy configuration
  will be used for making the enforcement decision.

  - `PolicyDefaults` `(array<ACLLink>)` - This is the list of default policies
    that should be applied to all tokens created in this namespace. The ACLLink
    struct is an object with an "ID" and/or "Name" field to identify a policy. 
    When a name is used instead of an ID, Consul will resolve the name to an ID
    and store that internally.
    
   - `RoleDefaults` `(array<ACLLink>)` - This is the list of default roles
    that should be applied to all tokens created in this namespace. The ACLLink
    struct is an object with an "ID" and/or "Name" field to identify a policy. 
    When a name is used instead of an ID, Consul will resolve the name to an ID
    and store that internally.

- `Meta` `(map<string|string>: <optional>)` - Specifies arbitrary KV metadata
  to associate with the namespace.

### Sample Payload

```json
{
   "Description": "Namespace for Team 1",
   "ACLs": {
      "PolicyDefaults": [
         {
            "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1"
         },
         {
            "Name": "node-read"
         }
      ],
      "RoleDefaults": [
         {
            "ID": "69748856-ae69-d620-3ec4-07844b3c6be7"
         },
         {
            "Name": "ns-team-2-read"
         }
      ]
   },
   "Meta": {
      "foo": "bar"
   }
}
```

### Sample Request

```sh
$ curl -X PUT \
   -H "X-Consul-Token: 5cdcae6c-0cce-4210-86fe-5dff3b984a6e" \
   --data @payload.json \
   http://127.0.0.1:8500/v1/namespace/team-1
```

### SampleResponse

```json
{
    "Name": "team-1",
    "Description": "Namespace for Team 1",
    "ACLs": {
        "PolicyDefaults": [
            {
                "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1",
                "Name": "service-read"
            },
            {
                "ID": "af937401-9950-fcae-8396-610ce047649a",
                "Name": "node-read"
            }
        ],
        "RoleDefaults": [
            {
                "ID": "69748856-ae69-d620-3ec4-07844b3c6be7",
                "Name": "service-discovery"
            },
            {
                "ID": "ae4b3542-d824-eb5f-7799-3fd657847e4e",
                "Name": "ns-team-2-read"
            }
        ]
    },
    "Meta": {
        "foo": "bar"
    },
    "CreateIndex": 55,
    "ModifyIndex": 55
}
```

## Delete a Namespace

This endpoint marks a Namespace for deletion. Once marked Consul will
deleted all the associated Namespaced data in the background. Only once
all associated data has been deleted will the Namespace actually disappear.
Until then, further reads can be performed on the namespace and a `DeletedAt`
field will now be populated with the timestamp of when the Namespace was
marked for deletion.

| Method   | Path                      | Produces                   |
| -------- | ------------------------- | -------------------------- |
| `DELETE` | `/namespace/:name`  | N/A        |

This endpoint will return no data. Success or failure is indicated by the status
code returned.

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `operator:write`  |

### Parameters

- `name` `(string: <required>)` - Specifies the namespace to delete. This
  is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X DELETE \
   -H "X-Consul-Token: b23b3cad-5ea1-4413-919e-c76884b9ad60" \
   http://127.0.0.1:8500/v1/namespace/team-1
```

### Sample Read Output After Deletion Prior to Removal

```json
{
    "Name": "team-1",
    "Description": "Namespace for Team 1",
    "ACLs": {
        "PolicyDefaults": [
            {
                "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1",
                "Name": "service-read"
            },
            {
                "ID": "af937401-9950-fcae-8396-610ce047649a",
                "Name": "node-read"
            }
        ],
        "RoleDefaults": [
            {
                "ID": "69748856-ae69-d620-3ec4-07844b3c6be7",
                "Name": "service-discovery"
            },
            {
                "ID": "ae4b3542-d824-eb5f-7799-3fd657847e4e",
                "Name": "ns-team-2-read"
            }
        ]
    },
    "Meta": {
        "foo": "bar"
    },
    "DeletedAt": "2019-12-02T23:00:00Z",
    "CreateIndex": 55,
    "ModifyIndex": 100
}
```

## List all Namespaces

This endpoint lists all the Namespaces. The output will be filtered based on the 
privileges of the ACL token used for the request.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/namespaces`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `operator:read` or `namespace:*:read`<sup>1</sup>   |

<sup>1</sup> Access can be granted to list the Namespace if the token used when making 
the request has been granted any access in the namespace (read, list or write).

### Sample Request

```sh
$ curl -H "X-Consul-Token: 0137db51-5895-4c25-b6cd-d9ed992f4a52" \
   http://127.0.0.1:8500/v1/namespaces
```

### Sample Response

```json
[
    {
        "Name": "default",
        "Description": "Builtin Default Namespace",
        "CreateIndex": 6,
        "ModifyIndex": 6
    },
    {
        "Name": "team-1",
        "Description": "Namespace for Team 1",
        "ACLs": {
            "PolicyDefaults": [
                {
                    "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1",
                    "Name": "service-read"
                },
                {
                    "ID": "af937401-9950-fcae-8396-610ce047649a",
                    "Name": "node-read"
                }
            ],
            "RoleDefaults": [
                {
                    "ID": "69748856-ae69-d620-3ec4-07844b3c6be7",
                    "Name": "service-discovery"
                },
                {
                    "ID": "ae4b3542-d824-eb5f-7799-3fd657847e4e",
                    "Name": "ns-team-2-read"
                }
            ]
        },
        "Meta": {
            "foo": "bar"
        },
        "CreateIndex": 55,
        "ModifyIndex": 55
    }
]
```
