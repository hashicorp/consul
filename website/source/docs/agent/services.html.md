---
layout: "docs"
page_title: "Service Definition"
sidebar_current: "docs-agent-services"
description: |-
  One of the main goals of service discovery is to provide a catalog of available services. To that end, the agent provides a simple service definition format to declare the availability of a service and to potentially associate it with a health check. A health check is considered to be application level if it is associated with a service. A service is defined in a configuration file or added at runtime over the HTTP interface.
---

# Services

One of the main goals of service discovery is to provide a catalog of available
services. To that end, the agent provides a simple service definition format
to declare the availability of a service and to potentially associate it with
a health check. A health check is considered to be application level if it is
associated with a service. A service is defined in a configuration file
or added at runtime over the HTTP interface.

## Service Definition

To configure a service, either provide the service definition as a `-config-file` option to
the agent or place it inside the `-config-dir` of the agent. The file
must end in the `.json` or `.hcl` extension to be loaded by Consul. Check
definitions can be updated by sending a `SIGHUP` to the agent.
Alternatively, the service can be registered dynamically using the [HTTP
API](/api/index.html).

A service definition is a configuration that looks like the following. This
example shows all possible fields, but note that only a few are required.

```javascript
{
  "service": {
    "name": "redis",
    "tags": ["primary"],
    "address": "",
    "meta": {
      "meta": "for my service"
    },
    "port": 8000,
    "enable_tag_override": false,
    "checks": [
      {
        "args": ["/usr/local/bin/check_redis.py"],
        "interval": "10s"
      }
    ],
    "connect": {
      "native": false,
      "proxy": {
        "command": [],
        "config": {}
      }
    }
  }
}
```

A service definition must include a `name` and may optionally provide an
`id`, `tags`, `address`, `port`, `check`, `meta` and `enable_tag_override`.
The `id` is set to the `name` if not provided. It is required that all
services have a unique ID per node, so if names might conflict then
unique IDs should be provided.

For Consul 0.9.3 and earlier you need to use `enableTagOverride`. Consul 1.0
supports both `enable_tag_override` and `enableTagOverride` but the latter is
deprecated and has been removed in Consul 1.1.

The `tags` property is a list of values that are opaque to Consul but
can be used to distinguish between `primary` or `secondary` nodes,
different versions, or any other service level labels.

The `address` field can be used to specify a service-specific IP address. By
default, the IP address of the agent is used, and this does not need to be provided.
The `port` field can be used as well to make a service-oriented architecture
simpler to configure; this way, the address and port of a service can
be discovered.

The `meta` object is a map of max 64 key/values with string semantics. Key can contain
only ASCII chars and no special characters (`A-Z` `a-z` `0-9` `_` and `-`).
For performance and security reasons, values as well as keys are limited to 128
characters for keys, 512 for values. This object has the same limitations as the node
meta object in node definition.
All those meta data can be retrieved individually per instance of the service
and all the instances of a given service have their own copy of it.

Services may also contain a `token` field to provide an ACL token. This token is
used for any interaction with the catalog for the service, including
[anti-entropy syncs](/docs/internals/anti-entropy.html) and deregistration.

A service can have an associated health check. This is a powerful feature as
it allows a web balancer to gracefully remove failing nodes, a database
to replace a failed secondary, etc. The health check is strongly integrated in
the DNS interface as well. If a service is failing its health check or a
node has any failing system-level check, the DNS interface will omit that
node from any service query.

The check must be of the script, HTTP, TCP or TTL type. If it is a script type,
`args` and `interval` must be provided. If it is a HTTP type, `http` and
`interval` must be provided. If it is a TCP type, `tcp` and `interval` must be
provided. If it is a TTL type, then only `ttl` must be provided. The check name
is automatically generated as `service:<service-id>`. If there are multiple
service checks registered, the ID will be generated as
`service:<service-id>:<num>` where `<num>` is an incrementing number starting
from `1`.

-> **Note:** There is more information about [checks here](/docs/agent/checks.html).

The `enable_tag_override` can optionally be specified to disable the
anti-entropy feature for this service. If `enable_tag_override` is set to
`TRUE` then external agents can update this service in the
[catalog](/api/catalog.html) and modify the tags. Subsequent
local sync operations by this agent will ignore the updated tags. For
example, if an external agent modified both the tags and the port for
this service and `enable_tag_override` was set to `TRUE` then after the next
sync cycle the service's port would revert to the original value but the
tags would maintain the updated value. As a counter example: If an
external agent modified both the tags and port for this service and
`enable_tag_override` was set to `FALSE` then after the next sync cycle the
service's port *and* the tags would revert to the original value and all
modifications would be lost.

It's important to note that this applies only to the locally registered
service. If you have multiple nodes all registering the same service
their `enable_tag_override` configuration and all other service
configuration items are independent of one another. Updating the tags
for the service registered on one node is independent of the same
service (by name) registered on another node. If `enable_tag_override` is
not specified the default value is false. See [anti-entropy
syncs](/docs/internals/anti-entropy.html) for more info.

For Consul 0.9.3 and earlier you need to use `enableTagOverride`. Consul 1.0
supports both `enable_tag_override` and `enableTagOverride` but the latter is
deprecated and has been removed as of Consul 1.1.

The `connect` field can be specified to configure [Connect](/docs/connect/index.html)
for a service. This field is available in Consul 1.2 and later. The `native`
value can be set to true to advertise the service as
[Connect-native](/docs/connect/native.html). If the `proxy` field is set
(even to an empty object), then this will enable a
[managed proxy](/docs/connect/proxies.html) for the service. The fields within
`proxy` are used to configure the proxy and are specified in the
[proxy docs](/docs/connect/proxies.html).

## Multiple Service Definitions

Multiple services definitions can be provided at once using the plural
`services` key in your configuration file.

```javascript
{
  "services": [
    {
      "id": "red0",
      "name": "redis",
      "tags": [
        "primary"
      ],
      "address": "",
      "port": 6000,
      "checks": [
        {
          "args": ["/bin/check_redis", "-p", "6000"],
          "interval": "5s",
          "ttl": "20s"
        }
      ]
    },
    {
      "id": "red1",
      "name": "redis",
      "tags": [
        "delayed",
        "secondary"
      ],
      "address": "",
      "port": 7000,
      "checks": [
        {
          "args": ["/bin/check_redis", "-p", "7000"],
          "interval": "30s",
          "ttl": "60s"
        }
      ]
    },
    ...
  ]
}
```

## Service and Tag Names with DNS

Consul exposes service definitions and tags over the [DNS](/docs/agent/dns.html)
interface. DNS queries have a strict set of allowed characters and a
well-defined format that Consul cannot override. While it is possible to
register services or tags with names that don't match the conventions, those
services and tags will not be discoverable via the DNS interface. It is
recommended to always use DNS-compliant service and tag names.

DNS-compliant service and tag names may contain any alpha-numeric characters, as
well as dashes. Dots are not supported because Consul internally uses them to
delimit service tags.
