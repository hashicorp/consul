---
layout: api
page_title: Filtering
sidebar_current: api-features-filtering
description: |-
  Consul exposes a RESTful HTTP API to control almost every aspect of the
  Consul agent.
---

# Filtering

A filter expression is used to refine a data query for some API listing endpoints as notated in the individual API documentation.
Filtering will be executed on the Consul server before data is returned, reducing the network load. To pass a
filter expression to Consul, with a data query, use the `filter` parameter.

```sh
curl -G <path> --data-urlencode 'filter=<filter expression>'
```

To create a filter expression, you will write one or more expressions using matching operators, selectors, and values.

## Expression Syntax

Expressions are written in plain text format. Boolean logic and parenthesization are
supported. In general whitespace is ignored, except within literal
strings.

### Expressions

There are several methods for connecting expressions, including

- logical `or`
- logical `and`
- logical `not`
- grouping with parenthesis
- matching expressions

```text
// Logical Or - evaluates to true if either sub-expression does
<Expression 1> or <Expression 2>

// Logical And - evaluates to true if both sub-expressions do
<Expression 1 > and <Expression 2>

// Logical Not - evaluates to true if the sub-expression does not
not <Expression 1>

// Grouping - Overrides normal precedence rules
( <Expression 1> )

// Inspects data to check for a match
<Matching Expression 1>
```

Standard operator precedence can be expected for the various forms. For
example, the following two expressions would be equivalent.

```text
<Expression 1> and not <Expression 2> or <Expression 3>

( <Expression 1> and (not <Expression 2> )) or <Expression 3>
```

### Matching Operators

Matching operators are used to create an expression. All matching operators use a selector or value to choose what data should be
matched. Each endpoint that supports filtering accepts a potentially
different list of selectors and is detailed in the API documentation for
those endpoints.


```text
// Equality & Inequality checks
<Selector> == <Value>
<Selector> != <Value>

// Emptiness checks
<Selector> is empty
<Selector> is not empty

// Contains checks or Substring Matching
<Value> in <Selector>
<Value> not in <Selector>
<Selector> contains <Value>
<Selector> not contains <Value>

// Regular Expression Matching
<Selector> matches <Value>
<Selector> not matches <Value>
```

### Selectors

Selectors are used by matching operators to create an expression. They are
defined by a `.` separated list of names. Each name must start with
a an ASCII letter and can contain ASCII letters, numbers, and underscores. When
part of the selector references a map value it may be expressed using the form
`["<map key name>"]` instead of `.<map key name>`. This allows the possibility
of using map keys that are not valid selectors in and of themselves.

```text
// selects the foo key within the ServiceMeta mapping for the
// /catalog/service/:service endpoint
ServiceMeta.foo

// Also selects the foo key for the same endpoint
ServiceMeta["foo"]
```

### Values

Values are used by matching operators to create an expression. Values can be any valid selector, a number, or a quoted string. For numbers any
base 10 integers and floating point numbers are possible. For quoted strings,
they may either be enclosed in double quotes or backticks. When enclosed in
backticks they are treated as raw strings and escape sequences such as `\n`
will not be expanded.

## Filter Utilization

Generally, only the main object is filtered. When filtering for
an item within an array that is not at the top level, the entire array that contains the item
will be returned. This is usually the outermost object of a response,
but in some cases such the [`/catalog/node/:node`](/api/catalog.html#list-services-for-node)
endpoint the filtering is performed on a object embedded within the results.

### Performance

Filters are executed on the servers and therefore will consume some amount
of CPU time on the server. For non-stale queries this means that the filter
is executed on the leader.

### Filtering Examples

#### Agent API

**Command - Unfiltered**

```sh
curl -X GET localhost:8500/v1/agent/services
```

**Response - Unfiltered**

```json
{
    "redis1": {
        "ID": "redis1",
        "Service": "redis",
        "Tags": [
            "primary",
            "production"
        ],
        "Meta": {
            "env": "production",
            "foo": "bar"
        },
        "Port": 1234,
        "Address": "",
        "Weights": {
            "Passing": 1,
            "Warning": 1
        },
        "EnableTagOverride": false
    },
    "redis2": {
        "ID": "redis2",
        "Service": "redis",
        "Tags": [
            "secondary",
            "production"
        ],
        "Meta": {
            "env": "production",
            "foo": "bar"
        },
        "Port": 1235,
        "Address": "",
        "Weights": {
            "Passing": 1,
            "Warning": 1
        },
        "EnableTagOverride": false
    },
    "redis3": {
        "ID": "redis3",
        "Service": "redis",
        "Tags": [
            "primary",
            "qa"
        ],
        "Meta": {
            "env": "qa"
        },
        "Port": 1234,
        "Address": "",
        "Weights": {
            "Passing": 1,
            "Warning": 1
        },
        "EnableTagOverride": false
    }
}
```

**Command - Filtered**

```sh
curl -G localhost:8500/v1/agent/services --data-urlencode 'filter=Meta.env == qa'
```

**Response - Filtered**

```json
{
    "redis3": {
        "ID": "redis3",
        "Service": "redis",
        "Tags": [
            "primary",
            "qa"
        ],
        "Meta": {
            "env": "qa"
        },
        "Port": 1234,
        "Address": "",
        "Weights": {
            "Passing": 1,
            "Warning": 1
        },
        "EnableTagOverride": false
    }
}
```

#### Catalog API

**Command - Unfiltered**

```sh
curl -X GET localhost:8500/v1/catalog/service/api-internal
```

**Response - Unfiltered**

```json
[
    {
        "ID": "b4f64e8c-5c7d-11e9-bf68-8c8590bd0966",
        "Node": "node-1",
        "Address": "198.18.0.1",
        "Datacenter": "dc1",
        "TaggedAddresses": null,
        "NodeMeta": {
            "agent": "true",
            "arch": "i386",
            "os": "darwin"
        },
        "ServiceKind": "",
        "ServiceID": "api-internal",
        "ServiceName": "api-internal",
        "ServiceTags": [
            "tag"
        ],
        "ServiceAddress": "",
        "ServiceWeights": {
            "Passing": 1,
            "Warning": 1
        },
        "ServiceMeta": {
            "environment": "qa"
        },
        "ServicePort": 9090,
        "ServiceEnableTagOverride": false,
        "ServiceProxy": {},
        "ServiceConnect": {},
        "CreateIndex": 30,
        "ModifyIndex": 30
    },
    {
        "ID": "b4faf93a-5c7d-11e9-840d-8c8590bd0966",
        "Node": "node-2",
        "Address": "198.18.0.2",
        "Datacenter": "dc1",
        "TaggedAddresses": null,
        "NodeMeta": {
            "arch": "arm",
            "os": "linux"
        },
        "ServiceKind": "",
        "ServiceID": "api-internal",
        "ServiceName": "api-internal",
        "ServiceTags": [
            "test",
            "tag"
        ],
        "ServiceAddress": "",
        "ServiceWeights": {
            "Passing": 1,
            "Warning": 1
        },
        "ServiceMeta": {
            "environment": "production"
        },
        "ServicePort": 9090,
        "ServiceEnableTagOverride": false,
        "ServiceProxy": {},
        "ServiceConnect": {},
        "CreateIndex": 29,
        "ModifyIndex": 29
    },
    {
        "ID": "b4fbe7f4-5c7d-11e9-ac82-8c8590bd0966",
        "Node": "node-4",
        "Address": "198.18.0.4",
        "Datacenter": "dc1",
        "TaggedAddresses": null,
        "NodeMeta": {
            "arch": "i386",
            "os": "freebsd"
        },
        "ServiceKind": "",
        "ServiceID": "api-internal",
        "ServiceName": "api-internal",
        "ServiceTags": [],
        "ServiceAddress": "",
        "ServiceWeights": {
            "Passing": 1,
            "Warning": 1
        },
        "ServiceMeta": {
            "environment": "qa"
        },
        "ServicePort": 9090,
        "ServiceEnableTagOverride": false,
        "ServiceProxy": {},
        "ServiceConnect": {},
        "CreateIndex": 28,
        "ModifyIndex": 28
    }
]
```

**Command - Filtered**

```sh
curl -G localhost:8500/v1/catalog/service/api-internal --data-urlencode 'filter=NodeMeta.os == linux'
```

**Response - Filtered**

```json
[
    {
        "ID": "b4faf93a-5c7d-11e9-840d-8c8590bd0966",
        "Node": "node-2",
        "Address": "198.18.0.2",
        "Datacenter": "dc1",
        "TaggedAddresses": null,
        "NodeMeta": {
            "arch": "arm",
            "os": "linux"
        },
        "ServiceKind": "",
        "ServiceID": "api-internal",
        "ServiceName": "api-internal",
        "ServiceTags": [
            "test",
            "tag"
        ],
        "ServiceAddress": "",
        "ServiceWeights": {
            "Passing": 1,
            "Warning": 1
        },
        "ServiceMeta": {
            "environment": "production"
        },
        "ServicePort": 9090,
        "ServiceEnableTagOverride": false,
        "ServiceProxy": {},
        "ServiceConnect": {},
        "CreateIndex": 29,
        "ModifyIndex": 29
    }
]

```

#### Health API

**Command - Unfiltered**

```sh
curl -X GET localhost:8500/v1/health/node/node-1
```

**Response - Unfiltered**

```json
[
    {
        "Node": "node-1",
        "CheckID": "node-health",
        "Name": "Node level check",
        "Status": "critical",
        "Notes": "",
        "Output": "",
        "ServiceID": "",
        "ServiceName": "",
        "ServiceTags": [],
        "Definition": {},
        "CreateIndex": 13,
        "ModifyIndex": 13
    },
    {
        "Node": "node-1",
        "CheckID": "svc-web-health",
        "Name": "Service level check - web",
        "Status": "warning",
        "Notes": "",
        "Output": "",
        "ServiceID": "",
        "ServiceName": "web",
        "ServiceTags": [],
        "Definition": {},
        "CreateIndex": 18,
        "ModifyIndex": 18
    }
]
```

**Command - Filtered**

```sh
curl -G localhost:8500/v1/health/node/node-1 --data-urlencode 'filter=ServiceName != ""'
```

**Response - Filtered**

```json
[
    {
        "Node": "node-1",
        "CheckID": "svc-web-health",
        "Name": "Service level check - web",
        "Status": "warning",
        "Notes": "",
        "Output": "",
        "ServiceID": "",
        "ServiceName": "web",
        "ServiceTags": [],
        "Definition": {},
        "CreateIndex": 18,
        "ModifyIndex": 18
    }
]
```
