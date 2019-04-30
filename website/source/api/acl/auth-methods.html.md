---
layout: api
page_title: ACL Auth Methods - HTTP API
sidebar_current: api-acl-auth-methods
description: |-
  The /acl/auth-method endpoints manage Consul's ACL Auth Methods.
---

-> **1.5.0+:**  The auth method APIs are available in Consul versions 1.5.0 and newer.

# ACL Auth Method HTTP API

The `/acl/auth-method` endpoints [create](#create-an-auth-method),
[read](#read-an-auth-method), [update](#update-an-auth-method),
[list](#list-auth-methods) and [delete](#delete-an-auth-method)
ACL auth methods in Consul.

For more information on how to setup ACLs, please see
the [ACL Guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/production-acls).

## Create an Auth Method

This endpoint creates a new ACL auth method.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/auth-method`           | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `Name` `(string: <required>)` - Specifies a name for the ACL auth method. The
  name can contain alphanumeric characters, dashes `-`, and  underscores `_`.
  This field is immutable and must be unique.
   
- `Type` `(string: <required>)` - The type of auth method being configured.
  The only allowed value in Consul 1.5.0 is `"kubernetes"`. This field is
  immutable.

- `Description` `(string: "")` - Free form human readable description of the
  auth method.

- `Config` `(map[string]string: <required>)` - The raw configuration to use for
  the chosen auth method. Contents will vary depending upon the type chosen.
  For more information on configuring specific auth method types, see the [auth
  method documentation](/docs/acl/acl-auth-methods.html).

### Sample Payload

```json
{
    "Name": "minikube",
    "Type": "kubernetes",
    "Description": "dev minikube cluster",
    "Config": {
        "Host": "https://192.0.2.42:8443",
        "CACert": "-----BEGIN CERTIFICATE-----\n...-----END CERTIFICATE-----\n",
        "ServiceAccountJWT": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9..."
    }
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/auth-method
```

### Sample Response

```json
{
    "Name": "minikube",
    "Type": "kubernetes",
    "Description": "dev minikube cluster",
    "Config": {
        "Host": "https://192.0.2.42:8443",
        "CACert": "-----BEGIN CERTIFICATE-----\n...-----END CERTIFICATE-----\n",
        "ServiceAccountJWT": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9..."
    },
    "CreateIndex": 15,
    "ModifyIndex": 15
}
```

## Read an Auth Method

This endpoint reads an ACL auth method with the given name. If no
auth method exists with the given name, a 404 is returned instead of a
200 response.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/auth-method/:name`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

### Parameters

- `name` `(string: <required>)` - Specifies the name of the ACL auth method to
  read. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/auth-method/minikube
```

### Sample Response

```json
{
    "Name": "minikube",
    "Type": "kubernetes",
    "Description": "dev minikube cluster",
    "Config": {
        "Host": "https://192.0.2.42:8443",
        "CACert": "-----BEGIN CERTIFICATE-----\n...-----END CERTIFICATE-----\n",
        "ServiceAccountJWT": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9..."
    },
    "CreateIndex": 15,
    "ModifyIndex": 224
}
```

## Update an Auth Method

This endpoint updates an existing ACL auth method.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/auth-method/:name`     | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `Name` `(string: <required>)` - Specifies the name of the auth method to
  update. This is required in the URL path but may also be specified in the
  JSON body. If specified in both places then they must match exactly.

- `Type` `(string: <required>)` - Specifies the type of the auth method being
  updated.  This field is immutable so if present in the body then it must
  match the existing value. If not present then the value will be filled in by
  Consul.

- `Description` `(string: "")` - Free form human readable description of the
  auth method.

- `Config` `(map[string]string: <required>)` - The raw configuration to use for
  the chosen auth method. Contents will vary depending upon the type chosen.
  For more information on configuring specific auth method types, see the [auth
  method documentation](/docs/acl/acl-auth-methods.html).

### Sample Payload

```json
{
    "Name": "minikube",
    "Description": "updated name",
    "Config": {
        "Host": "https://192.0.2.42:8443",
        "CACert": "-----BEGIN CERTIFICATE-----\n...-----END CERTIFICATE-----\n",
        "ServiceAccountJWT": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9..."
    }
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/auth-method/minikube
```

### Sample Response

```json
{
    "Name": "minikube",
    "Description": "updated name",
    "Type": "kubernetes",
    "Config": {
        "Host": "https://192.0.2.42:8443",
        "CACert": "-----BEGIN CERTIFICATE-----\n...-----END CERTIFICATE-----\n",
        "ServiceAccountJWT": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9..."
    },
    "CreateIndex": 15,
    "ModifyIndex": 224
}
```

## Delete an Auth Method

This endpoint deletes an ACL auth method.

~> Deleting an auth method will also immediately delete all associated
[binding rules](/api/acl/binding-rules.html) as well as any
outstanding [tokens](/api/acl/tokens.html) created from this auth method.

| Method   | Path                      | Produces                   |
| -------- | ------------------------- | -------------------------- |
| `DELETE` | `/acl/auth-method/:name`  | `application/json`         |

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

- `name` `(string: <required>)` - Specifies the name of the ACL auth method to
  delete. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X DELETE \
    http://127.0.0.1:8500/v1/acl/auth-method/minikube
```

### Sample Response

```json
true
```

## List Auth Methods

This endpoint lists all the ACL auth methods.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/auth-methods`          | `application/json`         |

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
$ curl -X GET http://127.0.0.1:8500/v1/acl/auth-methods
```

### Sample Response

-> **Note** - The contents of the `Config` field are not included in the
listing and must be retrieved by the [auth method reading endpoint](#read-an-auth-method).

```json
[
    {
        "Name": "minikube-1",
        "Type": "kubernetes",
        "Description": "",
        "CreateIndex": 14,
        "ModifyIndex": 14
    },
    {
        "Name": "minikube-2",
        "Type": "kubernetes",
        "Description": "",
        "CreateIndex": 15,
        "ModifyIndex": 15
    }
]
```
