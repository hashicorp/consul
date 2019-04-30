---
layout: api
page_title: ACL Binding Rules - HTTP API
sidebar_current: api-acl-binding-rules
description: |-
  The /acl/binding-rule endpoints manage Consul's ACL Binding Rules.
---

-> **1.5.0+:** The binding rule APIs are available in Consul versions 1.5.0 and newer.

# ACL Binding Rule HTTP API

The `/acl/binding-rule` endpoints [create](#create-a-binding-rule),
[read](#read-a-binding-rule), [update](#update-a-binding-rule),
[list](#list-binding-rules) and [delete](#delete-a-binding-rule)  ACL binding
rules in Consul.

For more information on how to setup ACLs, please see
the [ACL Guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/production-acls).

## Create a Binding Rule

This endpoint creates a new ACL binding rule.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/binding-rule`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `Description` `(string: "")` - Free form human readable description of the binding rule.

- `AuthMethod` `(string: <required>)` - The name of the auth method that this
  rule applies to. This field is immutable.

- `Selector` `(string: "")` - Specifies the expression used to match this rule
  against valid identities returned from an auth method validation. If empty
  this binding rule matches all valid identities returned from the auth method. For example: 

    ```text
    serviceaccount.namespace==default and serviceaccount.name!=vault
    ```

- `BindType` `(string: <required>)` - Specifies the way the binding rule
  affects a token created at login. 
  
  - `BindType=service` - The computed bind name value is used as an
    `ACLServiceIdentity.ServiceName` field in the token that is created.

        ```json
        { ...other fields...
            "ServiceIdentities": [
                { "ServiceName": "<computed BindName>" }
            ]
        }
        ```

  - `BindType=role` - The computed bind name value is used as a `RoleLink.Name`
    field in the token that is created. This binding rule will only apply if a
    role with the given name exists at login-time. If it does not then this
    rule is ignored.

        ```json
        { ...other fields...
            "Roles": [
                { "Name": "<computed BindName>" }
            ]
        }
        ```

- `BindName` `(string: <required>)` - The name to bind to a token at
  login-time.  What it binds to can be adjusted with different values of the
  `BindType` field. This can either be a plain string or lightly templated
  using [HIL syntax](https://github.com/hashicorp/hil) to interpolate the same
  values that are usable by the `Selector` syntax. For example:
  
    ```text
    prefixed-${serviceaccount.name}
    ```

### Sample Payload

```json
{
    "Description": "example rule",
    "AuthMethod": "minikube",
    "Selector": "serviceaccount.namespace==default",
    "BindType": "service",
    "BindName": "{{ serviceaccount.name }}"
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/binding-rule
```

### Sample Response

```json
{
    "ID": "000ed53c-e2d3-e7e6-31a5-c19bc3518a3d",
    "Description": "example rule",
    "AuthMethod": "minikube",
    "Selector": "serviceaccount.namespace==default",
    "BindType": "service",
    "BindName": "{{ serviceaccount.name }}",
    "CreateIndex": 17,
    "ModifyIndex": 17
}
```

## Read a Binding Rule

This endpoint reads an ACL binding rule with the given ID. If no
binding rule exists with the given ID, a 404 is returned instead of a 200
response.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/binding-rule/:id`      | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

### Parameters

- `id` `(string: <required>)` - Specifies the UUID of the ACL binding rule
  to read. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/binding-rule/000ed53c-e2d3-e7e6-31a5-c19bc3518a3d
```

### Sample Response

```json
{
    "ID": "000ed53c-e2d3-e7e6-31a5-c19bc3518a3d",
    "Description": "example rule",
    "AuthMethod": "minikube",
    "Selector": "serviceaccount.namespace==default",
    "BindType": "service",
    "BindName": "{{ serviceaccount.name }}",
    "CreateIndex": 17,
    "ModifyIndex": 17
}
```

## Update a Binding Rule

This endpoint updates an existing ACL binding rule.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/acl/binding-rule/:id`      | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `NO`             | `none`            | `none`        | `acl:write`  |

### Parameters

- `ID` `(string: <required>)` - Specifies the ID of the binding rule to update.
  This is required in the URL path but may also be specified in the JSON body.
  If specified in both places then they must match exactly.

- `Description` `(string: "")` - Free form human readable description of the binding rule.

- `AuthMethod` `(string: <required>)` - Specifies the name of the auth
  method that this rule applies to. This field is immutable so if present in
  the body then it must match the existing value. If not present then the value
  will be filled in by Consul.

- `Selector` `(string: "")` - Specifies the expression used to match this rule
  against valid identities returned from an auth method validation. If empty
  this binding rule matches all valid identities returned from the auth method. For example: 

    ```text
    serviceaccount.namespace==default and serviceaccount.name!=vault
    ```

- `BindType` `(string: <required>)` - Specifies the way the binding rule
  affects a token created at login. 
  
  - `BindType=service` - The computed bind name value is used as an
    `ACLServiceIdentity.ServiceName` field in the token that is created.

        ```json
        { ...other fields...
            "ServiceIdentities": [
                { "ServiceName": "<computed BindName>" }
            ]
        }
        ```

  - `BindType=role` - The computed bind name value is used as a `RoleLink.Name`
    field in the token that is created. This binding rule will only apply if a
    role with the given name exists at login-time. If it does not then this
    rule is ignored.

        ```json
        { ...other fields...
            "Roles": [
                { "Name": "<computed BindName>" }
            ]
        }
        ```

- `BindName` `(string: <required>)` - The name to bind to a token at
  login-time.  What it binds to can be adjusted with different values of the
  `BindType` field. This can either be a plain string or lightly templated
  using [HIL syntax](https://github.com/hashicorp/hil) to interpolate the same
  values that are usable by the `Selector` syntax. For example:
  
    ```text
    prefixed-${serviceaccount.name}
    ```

### Sample Payload

```json
{
    "Description": "updated rule",
    "Selector": "serviceaccount.namespace=dev",
    "BindType": "role",
    "BindName": "{{ serviceaccount.name }}"
}
```

### Sample Request

```sh
$ curl -X PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/acl/binding-rule/000ed53c-e2d3-e7e6-31a5-c19bc3518a3d
```

### Sample Response

```json
{
    "ID": "000ed53c-e2d3-e7e6-31a5-c19bc3518a3d",
    "Description": "updated rule",
    "AuthMethod": "minikube",
    "Selector": "serviceaccount.namespace=dev",
    "BindType": "role",
    "BindName": "{{ serviceaccount.name }}",
    "CreateIndex": 17,
    "ModifyIndex": 18
}
```

## Delete a Binding Rule

This endpoint deletes an ACL binding rule.

| Method   | Path                      | Produces                   |
| -------- | ------------------------- | -------------------------- |
| `DELETE` | `/acl/binding-rule/:id`   | `application/json`         |

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

- `id` `(string: <required>)` - Specifies the UUID of the ACL binding rule to
  delete. This is required and is specified as part of the URL path.

### Sample Request

```sh
$ curl -X DELETE \
    http://127.0.0.1:8500/v1/acl/binding-rule/000ed53c-e2d3-e7e6-31a5-c19bc3518a3d
```

### Sample Response
```json
true
```

## List Binding Rules

This endpoint lists all the ACL binding rules.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/acl/binding-rules`         | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required |
| ---------------- | ----------------- | ------------- | ------------ |
| `YES`            | `all`             | `none`        | `acl:read`   |

## Parameters

- `authmethod` `(string: "")` - Filters the binding rule list to those binding
  rules that are linked with the specific named auth method.

## Sample Request

```sh
$ curl -X GET http://127.0.0.1:8500/v1/acl/binding-rules
```

### Sample Response

```json
[
    {
        "ID": "000ed53c-e2d3-e7e6-31a5-c19bc3518a3d",
        "Description": "example 1",
        "AuthMethod": "minikube-1",
        "BindType": "service",
        "BindName": "k8s-{{ serviceaccount.name }}",
        "CreateIndex": 17,
        "ModifyIndex": 17
    },
    {
        "ID": "b4f0a0a3-69f2-7a4f-6bef-326034ace9fa",
        "Description": "example 2",
        "AuthMethod": "minikube-2",
        "Selector": "serviceaccount.namespace==default",
        "BindName": "k8s-{{ serviceaccount.name }}",
        "CreateIndex": 18,
        "ModifyIndex": 18
    }
]
```
