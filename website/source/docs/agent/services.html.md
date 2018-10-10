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
    "kind": "connect-proxy",
    "proxy_destination": "redis",
    "proxy": {
      "destination_service_name": "redis",
      "destination_service_id": "redis1",
      "local_service_name": "127.0.0.1",
      "local_service_port": 9090,
      "config": {},
      "upstreams": []
    }
    "connect": {
      "native": false,
      "proxy": {
        "command": [],
        "config": {}
      }
    },
    "weights": {
      "passing": 5,
      "warning": 1
    }
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

The `kind` field is used to optionally identify the service as an [unmanaged
Connect proxy](/docs/connect/proxies.html#unmanaged-proxies) instance with the
value `connect-proxy`. For typical non-proxy instances the `kind` field must be
omitted. The `proxy` field is also required for unmanaged proxy registrations
and is only valid if `kind` is `connect-proxy`. The only required `proxy` field
is `destination_service_name`. From version 1.2.0 to 1.3.0 this was specified
using `proxy_destination` which still works but is now deprecated. See the
[unmanaged proxy
configuration](/docs/connect/proxies.html#complete-configuration-example)
documentation for full details.

The `connect` field can be specified to configure
[Connect](/docs/connect/index.html) for a service. This field is available in
Consul 1.2 and later. The `native` value can be set to true to advertise the
service as [Connect-native](/docs/connect/native.html). If the `proxy` field is
set (even to an empty object), then this will enable a [managed
proxy](/docs/connect/proxies.html) for the service. The fields within `proxy`
are used to configure the proxy and are specified in the [proxy
docs](/docs/connect/proxies.html). If `native` is true, it is an error to also
specifiy a managed proxy instance.

The `weights` field is an optional field to specify the weight of a service in
DNS SRV responses. If this field is not specified, its default value is:
`"weights": {"passing": 1, "warning": 1}`.
When a service is `critical`, it is excluded from DNS responses.
Services with warning checks are in included in responses by default,
but excluded if the optional param `only_passing = true` is present in
agent DNS configuration or `?passing` is used via the API.
When DNS SRV requests are made, the response will include the weights
specified given the state of the service.
This allows some instances to be given higher weight if they have more capacity,
and optionally allows reducing load on services with checks in `warning` status
by giving passing instances a higher weight.

### Checks

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

## Service Definition Parameter Case

For historical reasons Consul's API uses `CamelCased` parameter names in
responses, however it's configuration file syntax borrowed from HCL uses
`snake_case`. For this reason the registration APIs accept both cases for
service definition parameters although APIs will return the listings using
`CamelCase`.