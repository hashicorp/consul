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

To configure a service, either provide the service definition as a
`-config-file` option to the agent or place it inside the `-config-dir` of the
agent. The file must end in the `.json` or `.hcl` extension to be loaded by
Consul. Check definitions can be updated by sending a `SIGHUP` to the agent.
Alternatively, the service can be registered dynamically using the [HTTP
API](/api/index.html).

A service definition is a configuration that looks like the following. This
example shows all possible fields, but note that only a few are required.

```javascript
{
  "service": {
    "id": "redis",
    "name": "redis",
    "tags": ["primary"],
    "address": "",
    "meta": {
      "meta": "for my service"
    },
    "tagged_addresses": {
      "lan": {
        "address": "192.168.0.55",
        "port": 8000,
      },
      "wan": {
        "address": "198.18.0.23",
        "port": 80
      }
    },
    "port": 8000,
    "enable_tag_override": false,
    "checks": [
      {
        "args": ["/usr/local/bin/check_redis.py"],
        "interval": "10s"
      }
    ],
    "kind": "connect-proxy",
    "proxy_destination": "redis", // Deprecated
    "proxy": {
      "destination_service_name": "redis",
      "destination_service_id": "redis1",
      "local_service_address": "127.0.0.1",
      "local_service_port": 9090,
      "config": {},
      "upstreams": [],
      "mesh_gateway": {
        "mode": "local"
      }
    },
    "connect": {
      "native": false,
      "sidecar_service": {}
      "proxy": {  // Deprecated
        "command": [],
        "config": {}
      }
    },
    "weights": {
      "passing": 5,
      "warning": 1
    },
    "token": "233b604b-b92e-48c8-a253-5f11514e4b50"
  }
}
```

A service definition must include a `name` and may optionally provide an
`id`, `tags`, `address`, `meta`, `port`, `enable_tag_override`,  and `check`.
The `id` is set to the `name` if not provided. It is required that all
services have a unique ID per node, so if names might conflict then
unique IDs should be provided.

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

### Connect

The `kind` field is used to optionally identify the service as a [Connect
proxy](/docs/connect/proxies.html) instance with the value `connect-proxy` or
a [Mesh Gateway](/docs/connect/mesh_gateway.html) instance with the value `mesh-gateway`. For
typical non-proxy instances the `kind` field must be omitted. The `proxy` field
is also required for Connect proxy registrations and is only valid if `kind` is
`connect-proxy`. The only required `proxy` field is `destination_service_name`.
For more detail please see [complete proxy configuration
example](/docs/connect/registration/service-registration.html#complete-configuration-example)

-> **Deprecation Notice:** From version 1.2.0 to 1.3.0, proxy destination was
specified using `proxy_destination` at the top level. This will continue to work
until at least 1.5.0 but it's highly recommended to switch to using
`proxy.destination_service_name`.

The `connect` field can be specified to configure
[Connect](/docs/connect/index.html) for a service. This field is available in
Consul 1.2.0 and later. The `native` value can be set to true to advertise the
service as [Connect-native](/docs/connect/native.html). The `sidecar_service`
field is an optional nested service definition its behavior and defaults are
described in [Sidecar Service
Registration](/docs/connect/registration/sidecar-service.html). If `native` is true,
it is an error to also specify a sidecar service registration.

-> **Deprecation Notice:** From version 1.2.0 to 1.3.0 during beta, Connect
supported "Managed" proxies which are specified with the `connect.proxy` field.
[Managed Proxies are deprecated](/docs/connect/proxies/managed-deprecated.html)
and the `connect.proxy` field will be removed in a future major release.

### Checks

A service can have an associated health check. This is a powerful feature as
it allows a web balancer to gracefully remove failing nodes, a database
to replace a failed secondary, etc. The health check is strongly integrated in
the DNS interface as well. If a service is failing its health check or a
node has any failing system-level check, the DNS interface will omit that
node from any service query.

There are several check types that have differing required options as
[documented here](/docs/agent/checks.html). The check name is automatically
generated as `service:<service-id>`. If there are multiple service checks
registered, the ID will be generated as `service:<service-id>:<num>` where
`<num>` is an incrementing number starting from `1`.

-> **Note:** There is more information about [checks here](/docs/agent/checks.html).

### DNS SRV Weights

The `weights` field is an optional field to specify the weight of a service in
DNS SRV responses. If this field is not specified, its default value is:
`"weights": {"passing": 1, "warning": 1}`. When a service is `critical`, it is
excluded from DNS responses. Services with warning checks are included in
responses by default, but excluded if the optional param `only_passing = true`
is present in agent DNS configuration or `?passing` is used via the API.

When DNS SRV requests are made, the response will include the weights specified
given the state of the service. This allows some instances to be given higher
weight if they have more capacity, and optionally allows reducing load on
services with checks in `warning` status by giving passing instances a higher
weight.

### Enable Tag Override and Anti-Entropy

Services may also contain a `token` field to provide an ACL token. This token is
used for any interaction with the catalog for the service, including
[anti-entropy syncs](/docs/internals/anti-entropy.html) and deregistration.

You can optionally disable the anti-entropy feature for this service using the
`enable_tag_override` flag. External agents can modify tags on services in the
catalog, so subsequent sync operations can either maintain tag modifications or
revert them. If `enable_tag_override` is set to `TRUE`, the next sync cycle may
revert some service properties, **but** the tags would maintain the updated value.
If `enable_tag_override` is set to `FALSE`, the next sync cycle will revert any
updated service properties, **including** tags, to their original value.

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
          "timeout": "20s"
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
          "timeout": "60s"
        }
      ]
    },
    ...
  ]
}
```

In HCL you can specify the plural `services` key (although not `service`) multiple times:

```hcl
services {
  id = "red0"
  name = "redis"
  tags = [
    "primary"
  ]
  address = ""
  port = 6000
  checks = [
    {
      args = ["/bin/check_redis", "-p", "6000"]
      interval = "5s"
      timeout = "20s"
    }
  ]
}
services {
  id = "red1"
  name = "redis"
  tags = [
    "delayed",
    "secondary"
  ]
  address = ""
  port = 7000
  checks = [
    {
      args = ["/bin/check_redis", "-p", "7000"]
      interval = "30s"
      timeout = "60s"
    }
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

## Service Definition Parameter Case

For historical reasons Consul's API uses `CamelCased` parameter names in
responses, however it's configuration file uses `snake_case` for both HCL and
JSON representations. For this reason the registration _HTTP APIs_ accept both
name styles for service definition parameters although APIs will return the
listings using `CamelCase`.

Note though that **all config file formats require
`snake_case` fields**. We always document service definition examples using
`snake_case` and JSON since this format works in both config files and API
calls.
