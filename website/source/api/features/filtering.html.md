---
layout: api
page_title: Filtering
sidebar_current: api-features-filtering
description: |-
  Consul exposes a RESTful HTTP API to control almost every aspect of the
  Consul agent.
---

# Filtering

Some listing endpoints support the use of a filter expression. 
Filter expressions can be used on a requested
set of data, prior to sending back a response. This reduces the amount of
data returned, which is helpful for reducing load, searching, and monitoring. 

A filter expression can be passed to Consul with the `filter` query parameter. For example:

```text
?filter --data [ Expression or Expression]
```

Note, generally, only the main object is filtered. When filtering for
an item within an array, the entire object that contains the full array
will be returned. This is usually the outermost object of a response,
but in some cases such the [`/catalog/node/:node`](api/catalog.html#list-services-for-node)
endpoint the filtering is performed on a object embedded within the results. 


## Syntax

Filtering expressions are text based. Boolean logic and parenthesization are
supported. In general whitespace is ignored, except within literal
strings. 

### Expression

An expression can take one of a few forms.

```
// Logical Or - evaluates to true if either sub-expression does
<Expression> or <Expression>

// Logical And - evaluates to true if both sub-expressions do
<Expression> and <Expression>

// Logical Not - evaluates to true if the sub-expression does not
not <Expression>

// Grouping - Overrides normal precedence rules
( <Expression> )

// Inspects data to check for a match
<Matching Expression>
```

Standard operator precedence can be expected for the various forms. For
example, the following two expressions would be equivalent.

```
<Expression 1> and not <Expression 2> or <Expression 3>

( <Expression 1> and (not <Expression 2> )) or <Expression 3>
```

### Matching Expressions

All matching expressions use a Selector to choose what data should be
matched. Each endpoint that supports filtering accepts a potentially
different list of selectors and is detailed in the API documentation of
those endpoints. For many matching operations a value is also required.

```
// Equality/Inequality checks
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

Selectors are a `.` separated list of names. Each name must start with
a an ASCII letter and can contain ASCII letters, numbers and underscores. When
part of the selector references a map value it may be expressed using the form
`["<map key name>"]` instead of `.<map key name>`. This allows the possibility
of using map keys that are not valid selectors in and of themselves.

A few examples of selectors are:

```
// selects the foo key within the ServiceMeta mapping for the
// /catalog/service/:service endpoint
ServiceMeta.foo

// Also selects the foo key for the same endpoint
ServiceMeta["foo"]
```

### Values

Values can be any valid Selector, a number or a quoted string. For numbers any
base 10 integers and floating point numbers are possible. For quoted strings,
they may either be enclosed in double quotes or backticks. When enclosed in
backticks they are treated as raw strings and escape sequences such as `\n`
will not be expanded.

## Performance

Filters are executed on the servers and therefore will consume some amount
of CPU time on the server. For non-stale queries this means that the filter
is executed on the leader.
