---
layout: "docs"
page_title: "Connect - Proxies"
sidebar_current: "docs-connect-proxies"
description: |-
  A Connect-aware proxy enables unmodified applications to use Connect. A per-service proxy sidecar transparently handles inbound and outbound service connections, automatically wrapping and verifying TLS connections.
---

# Connect Proxies

A Connect-aware proxy enables unmodified applications to use Connect.
A per-service proxy sidecar transparently handles inbound and outbound
service connections, automatically wrapping and verifying TLS connections.

When a proxy is used, the actual service being proxied should only accept
connections on a loopback address. This requires all external connections
to be established via the Connect protocol to provide authentication and
authorization.

-> **Deprecation Note:** Managed Proxies are deprecated as of Consul 1.3. See
[managed proxy deprecation](/docs/connect/proxies/managed-deprecated.html) for
more information. It's strongly recommended to switch to one of the approaches
listed on this page as soon as possible.

## Proxy Service Definitions

Connect proxies are registered using regular [service
definitions](/docs/agent/services.html). They can be registered both in config
files or via the API just like any other service.

Additionally, to reduce the amount of boilerplate needed for a sidecar proxy,
application service definitions may define inline [sidecar service
registrations](/docs/connect/proxies/sidecar-service.html) which are an
opinionated shorthand for a separate full proxy registration as described here.

To function as a Connect proxy, they must be declared as a proxy type and
provide information about the service they represent.

To declare a service as a proxy, the service definition must contain
the following fields:

  * `kind` (string) must be set to `connect-proxy`. This declares that the
    service is a proxy type.

  * `proxy.destination_service_name` (string) must be set to the service that
    this proxy is representing. Note that this replaces `proxy_destination` in
    versions 1.2.0 to 1.3.0.

  * `port` must be set so that other Connect services can discover the exact
    address for connections. `address` is optional if the service is being
    registered against an agent, since it'll inherit the node address.

Minimal Example:

```json
{
  "name": "redis-proxy",
  "kind": "connect-proxy",
  "proxy": {
    "destination_service_name": "redis"
  },
  "port": 8181
}
```

With this service registered, any Connect clients searching for a
Connect-capable endpoint for "redis" will find this proxy.

### Sidecar Proxy Fields

Most Connect proxies are deployed as "sidecars" which means they are co-located
with a single service instance which they represent and proxy all inbound
traffic to. In this case the following fields must may also be set:

  * `proxy.destination_service_id` (string) is set to the _id_ (and not the
    _name_ if they are different) of the specific service instance that is being
    proxied. The proxied service is assumed to be registered on the same agent
    although it's not strictly validated to allow for un-coordinated
    registrations.

  * `proxy.local_service_port` (string) must specify the port the proxy should use
    to connect to the _local_ service instance.

  * `proxy.local_service_address` (string) can be set to override the IP or
    hostname the proxy should use to connect to the _local_ service. Defaults to
    `127.0.0.1`.

### Complete Configuration Example

The following is a complete example showing all the options available when
registering a proxy instance.

```json
{
  "name": "redis-proxy",
  "kind": "connect-proxy",
  "proxy": {
    "destination_service_name": "redis",
    "destination_service_id": "redis1",
    "local_service_address": "127.0.0.1",
    "local_service_port": 9090,
    "config": {},
    "upstreams": []
  },
  "port": 8181
}
```

-> **Deprecation Notice:** From version 1.2.0 to 1.3.0, proxy destination was
specified using `proxy_destination` at the top level. This will continue to work
until at least 1.5.0 but it's highly recommended to switch to using
`proxy.destination_service_name`.

#### Proxy Parameters

 - `destination_service_name` `string: <required>` - Specifies the _name_ of the
   service this instance is proxying. Both side-car and centralized
   load-balancing proxies must specify this. It is used during service
   discovery to find the correct proxy instances to route to for a given service
   name.

 - `destination_service_id` `string: <optional>` - Specifies the _ID_ of a single
   specific service instance that this proxy is representing. This is only valid
   for side-car style proxies that run on the same node. It is assumed that the
   service instance is registered via the same Consul agent so the ID is unique
   and has no node qualifier. This is useful to show in tooling which proxy
   instance is a side-car for which application instance and will enable
   fine-grained analysis of the metrics coming from the proxy.

 - `local_service_address` `string: <optional>` - Specifies the address a side-car
   proxy should attempt to connect to the local application instance on.
   Defaults to 127.0.0.1.

 - `local_service_port` `int: <optional>` - Specifies the port a side-car
   proxy should attempt to connect to the local application instance on.
   Defaults to the port advertised by the service instance identified by
   `destination_service_id` if it exists otherwise it may be empty in responses.

 - `config` `object: <optional>` - Specifies opaque config JSON that will be
   stored and returned along with the service instance from future API calls.

 - `upstreams` `array<Upstream>: <optional>` - Specifies the upstream services
   this proxy should create listeners for. The format is defined in
   [Upstream Configuration Reference](#upstream-configuration-reference).

### Upstream Configuration Reference

The following example shows all possible upstream configuration parameters.

Note that in versions 1.2.0 to 1.3.0, managed proxy upstreams were specified
inside the opaque `connect.proxy.config` map. The format is almost unchanged
however managed proxy upstreams are now defined a level up in the
`connect.proxy.upstreams`. The old location is deprecated and will be
automatically converted into the new for an interim period before support is
dropped in a future major release. The only difference in format between the
upstream defintions is that the field `destination_datacenter` has been renamed
to `datacenter` to reflect that it's the discovery target and not necessarily
the same as the instance that will be returned in the case of a prepared query
that fails over to another datacenter.

Note that `snake_case` is used here as it works in both [config file and API
registrations](/docs/agent/services.html#service-definition-parameter-case).

```json
{
  "destination_type": "service",
  "destination_name": "redis",
  "datacenter": "dc1",
  "local_bind_address": "127.0.0.1",
  "local_bind_port": 1234,
  "config": {}
},
```

* `destination_name` `string: <required>` - Specifies the name of the service or
  prepared query to route connect to.
* `local_bind_port` `int: <required>` - Specifies the port to bind a local
  listener to for the application to make outbound connections to this upstream.
* `local_bind_address` `string: <optional>` - Specifies the address to bind a
  local listener to for the application to make outbound connections to this
  upstream. Defaults to `127.0.0.1`.
* `destination_type` `string: <optional>` - Speficied the type of discovery
  query to use to find an instance to connect to. Valid values are `service` or
  `prepared_query`. Defaults to `service`.
* `datacenter` `string: <optional>` - Specifies the datacenter to issue the
  discovery query too. Defaults to the local datacenter.
* `config` `object: <optional>` - Specifies opaque configuration options that
  will be provided to the proxy instance for this specific upstream. Can contain
  any valid JSON object. This might be used to configure proxy-specific features
  like timeouts or retries for the given upstream. See the [built-in proxy
  configuration
  reference](/docs/connect/configuration.html#built-in-proxy-options) for
  options available when using the built-in proxy. If using Envoy as a proxy,
  see [Envoy configuration
  reference](/docs/connect/configuration.html#envoy-options)


### Dynamic Upstreams

If an application requires dynamic dependencies that are only available
at runtime, it must currently [natively integrate](/docs/connect/native.html)
with Connect. After natively integrating, the HTTP API or
[DNS interface](/docs/agent/dns.html#connect-capable-service-lookups)
can be used.