---
layout: api
page_title: Filtering
sidebar_current: api-features-filtering
description: |-
  Consul exposes a RESTful HTTP API to control almost every aspect of the
  Consul agent.
---

# Filtering

A filter expression is used to refine a data query for the agent, heath, and catalog API list endpoints. Filtering will be executed
on the Consul server before data is returned, reducing load. To pass a
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
- Logical `not`
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

// Contains checks
<Value> in <Selector>
<Value> not in <Selector>
<Selector> contains <Value>
<Selector> not contains <Value>
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
an item within an array, that is not at the top level, the entire array that contains the item
will be returned. This is usually the outermost object of a response,
but in some cases such the [`/catalog/node/:node`](api/catalog.html#list-services-for-node)
endpoint the filtering is performed on a object embedded within the results.

### Performance

Filters are executed on the servers and therefore will consume some amount
of CPU time on the server. For non-stale queries this means that the filter
is executed on the leader.

### Filtering Examples

#### Agent API

#### Catalog API

#### Health API
