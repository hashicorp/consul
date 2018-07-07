---
layout: "docs"
page_title: "Connect - Intentions"
sidebar_current: "docs-connect-intentions"
description: |-
  Intentions define access control for services via Connect and are used to control which services may establish connections. Intentions can be managed via the API, CLI, or UI.
---

# Intentions

Intentions define access control for services via Connect and are used
to control which services may establish connections. Intentions can be
managed via the API, CLI, or UI.

Intentions are enforced by the [proxy](/docs/connect/proxies.html)
or [natively integrated application](/docs/connect/native.html) on
inbound connections. After verifying the TLS client certificate, the
[authorize API endpoint](#) is called which verifies the connection
is allowed by testing the intentions. If authorize returns false the
connection must be terminated.

The default intention behavior is defined by the default
[ACL policy](/docs/guides/acl.html). If the default ACL policy is "allow all",
then all Connect connections are allowed by default. If the default ACL policy
is "deny all", then all Connect connections are denied by default.

## Intention Basics

Intentions can be managed via the
[API](#),
[CLI](#),
or UI. Please see the respective documentation for each for full details
on options, flags, etc.
Below is an example of a basic intention to show the basic attributes
of an intention. The full data model of an intention can be found in the
[API documentation](#).

```
$ consul intention create -deny web db
Created: web => db (deny)
```

The intention above is a deny intention with a source of "web" and
destination of "db". This says that connections from web to db are not
allowed and the connection will be rejected.

When an intention is modified, existing connections will not be affected.
This means that changing a connection from "allow" to "deny" today
_will not_ kill the connection. Addressing this shortcoming is on
the near term roadmap for Consul.

### Wildcard Intentions

An intention source or destination may also be the special wildcard
value `*`. This matches _any_ value and is used as a catch-all. Example:

```
$ consul intention create -deny web '*'
Created: web => * (deny)
```

This example says that the "web" service cannot connect to _any_ service.

### Metadata

Arbitrary string key/value data may be associated with intentions. This
is unused by Consul but can be used by external systems or for visibility
in the UI.

```
$ consul intention create \
  -deny \
  -meta description='Hello there' \
  web db
...

$ consul intention get web db
Source:             web
Destination:        db
Action:             deny
ID:                 31449e02-c787-f7f4-aa92-72b5d9b0d9ec
Meta[description]:  Hello there
Created At:         Friday, 25-May-18 02:07:51 CEST
```

## Precedence and Match Order

Intentions are matched in an implicit order based on specificity, preferring
deny over allow. Specificity is determined by whether a value is an exact
specified value or is the wildcard value `*`.
The full precedence table is shown below and is evaluated
top to bottom, with larger numbers being evaluated first.

| Source Name | Destination Name | Precedence |
| ----------- | ---------------- | ---------- |
| Exact       | Exact            | 9          |
| `*`         | Exact            | 8          |
| Exact       | `*`              | 6          |
| `*`         | `*`              | 5          |

The precedence value can be read from the [API](/api/connect/intentions.html)
after an intention is created.
Precedence cannot be manually overridden today. This is a feature that will
be added in a later version of Consul.

In the case the two precedence values match, Consul will evaluate
intentions based on lexographical ordering of the destination then
source name. In practice, this is a moot point since authorizing a connection
has an exact source and destination value so its impossible for two
valid non-wildcard intentions to match.

The numbers in the table above are not stable. Their ordering will remain
fixed but the actual number values may change in the future.
The numbers are non-contiguous because there are
some unused values in the middle in preparation for a future version of
Consul supporting namespaces.

## Intention Management Permissions

Intention management can be protected by [ACLs](/docs/guides/acl.html).
Permissions for intentions are _destination-oriented_, meaning the ACLs
for managing intentions are looked up based on the destination value
of the intention, not the source.

Intention permissions are first inherited from `service` management permissions.
For example, the ACL below would allow _read_ access to intentions with a
destination starting with "web":

```hcl
service "web" {
  policy = "read"
}
```

ACLs may also specify service-specific intention permissions. In the example
below, the ACL token may register a "web"-prefixed service but _may not_ read or write
intentions:

```hcl
service "web" {
  policy = "read"
  intention = "deny"
}
```

## Performance and Intention Updates

The intentions for services registered with a Consul agent are cached
locally on that agent. They are then updated via a background blocking query
against the Consul servers.

Connect connection attempts require only local agent
communication for authorization and generally impose only impose microseconds
of latency to the connection. All actions in the data path of connections
require only local data to ensure minimal performance overhead.

Updates to intentions are propagated nearly instantly to agents since agents
maintain a continuous blocking query in the background for intention updates
for registered services.

Because all the intention data is cached locally, the agents can fail static.
Even if the agents are severed completely from the Consul servers, inbound
connection authorization continues to work for a configured amount of time.
Changes to intentions will not be picked up until the partition heals, but
will then automatically take effect when connectivity is restored.
