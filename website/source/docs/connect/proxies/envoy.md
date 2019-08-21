---
layout: "docs"
page_title: "Connect - Envoy Integration"
sidebar_current: "docs-connect-proxies-envoy"
description: |-
  Consul Connect has first-class support for configuring Envoy proxy.
---

# Envoy Integration

Consul Connect has first class support for using
[Envoy](https://www.envoyproxy.io) as a proxy. Consul configures Envoy by
optionally exposing a gRPC service on the local agent that serves [Envoy's xDS
configuration
API](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol).

Consul can configure Envoy sidecars to proxy http/1.1, http2 or gRPC traffic at
L7 or any other tcp-based protocol at L4. Prior to Consul 1.5.0 Envoy proxies
could only proxy tcp at L4.

Configuration of some [L7 features](/docs/connect/l7-traffic-management.html)
is possible via [configuration entries](/docs/agent/config_entries.html). If
you wish to use an Envoy feature not currently exposed through these config
entries as an interim solution, you can add [custom Envoy
configuration](#advanced-configuration) in the [proxy service
definition](/docs/connect/registration/service-registration.html) allowing you
to use the more powerful features of Envoy.

~> **Note:** When using Envoy with Consul and not using the [`consul connect envoy` command](/docs/commands/connect/envoy.html)
   Envoy must be run with the `--max-obj-name-len` option set to `256` or greater.

## Supported Versions

Consul's Envoy support was added in version 1.3.0. The following table shows
compatible Envoy versions.

| Consul Version | Compatible Envoy Versions |
|---|---|
| 1.5.2 and higher | 1.11.1 1.10.0, 1.9.1, 1.8.0† |
| 1.5.0, 1.5.1 | 1.9.1, 1.8.0† |
| 1.3.x, 1.4.x | 1.9.1, 1.8.0†, 1.7.0† |

!> **Security Note:** Envoy versions lower than 1.9.1 are vulnerable to
 [CVE-2019-9900](https://github.com/envoyproxy/envoy/issues/6434) and
 [CVE-2019-9901](https://github.com/envoyproxy/envoy/issues/6435). Both are
 related to HTTP request parsing and so only affect Consul Connect users if they
 have configured HTTP routing rules via the ["escape
 hatch"](#custom-configuration). Still, we recommend that you use the most
 recent supported Envoy for your Consul version where possible.

## Getting Started

To get started with Envoy and see a working example you can follow the [Using
Envoy with Connect](https://learn.hashicorp.com/consul/developer-segmentation/connect-envoy) guide.

## Configuration

Envoy proxies require two types of configuration: an initial _bootstrap
configuration_ and dynamic configuration that is discovered from a "management
server", in this case Consul.

The bootstrap configuration at a minimum needs to configure the proxy with an
identity (node id) and the location of it's local Consul agent from which it
discovers all of it's dynamic configuration. See [Bootstrap
Configuration](#bootstrap-configuration) for more details.

The dynamic configuration Consul Connect provides to each Envoy instance includes:

 - TLS certificates and keys to enable mutual authentication and keep certificates
   rotating.
 - Service-discovery results for upstreams to enable each sidecar proxy to load-balance
   outgoing connections.
 - L7 configuration including timeouts and protocol-specific options.

For more information on the parts of the Envoy proxy runtime configuration
that are currently controllable via Consul Connect see [Dynamic
Configuration](#dynamic-configuration).

We plan to enable more and more of Envoy's features through
Connect's first-class configuration over time, however some advanced users will
need additional control to configure Envoy in specific ways. To enable this, we
provide several ["escape hatch"](#advanced-configuration) options that allow
users to provide low-level raw Envoy config syntax for some sub-components in each
Envoy instance. This allows operators to have full control over and
responsibility for correctly configuring Envoy and ensuring version support etc.

## Bootstrap Configuration

Envoy requires an initial bootstrap configuration file. The easiest way to
create this is using the [`consul connect envoy`
command](/docs/commands/connect/envoy.html). The command can either output the
bootstrap configuration directly to stdout or can generate it and then `exec`
the Envoy binary as a convenience wrapper.

Because some Envoy configuration options like metrics and tracing sinks can only be
specified via the bootstrap configuration, Connect as of Consul 1.5.0 adds
the ability to control some parts of the bootstrap config via proxy
configuration options.

Users can add the following configuration items to the [global `proxy-defaults`
configuration entry](/docs/agent/config-entries/proxy-defaults.html) or override them directly in the `proxy.config` field
of a [proxy service
definition](/docs/connect/registration/service-registration.html) or
[`sidecar_service`](/docs/connect/registration/sidecar-service.html) block.

- `envoy_statsd_url` - A URL in the form `udp://ip:port` identifying a UDP
  StatsD listener that Envoy should deliver metrics to. For example, this may be
  `udp://127.0.0.1:8125` if every host has a local StatsD listener. In this case
  users can configure this property once in the [global `proxy-defaults`
configuration entry](/docs/agent/config-entries/proxy-defaults.html) for convenience. Currently, TCP is not supported.

    ~> **Note:** currently the url **must use an ip address** not a dns name due
    to the way Envoy is setup for StatsD.

    Users can also specify the whole parameter in the form `$ENV_VAR_NAME`, which
    will cause the `consul connect envoy` command to resolve the actual URL from
    the named environment variable when it runs. This, for example, allows each
    pod in a Kubernetes cluster to learn of a pod-specific IP address for StatsD
    when the Envoy instance is bootstrapped while still allowing global
    configuration of all proxies to use StatsD in the [global `proxy-defaults`
configuration entry](/docs/agent/config-entries/proxy-defaults.html). The env variable must contain a full valid URL
    value as specified above and nothing else. It is not currently possible to use
    environment variables as only part of the URL.

- `envoy_dogstatsd_url` - The same as `envoy_statsd_url` with the following
  differences in behavior:
    - Envoy will use dogstatsd tags instead of statsd dot-separated metric names.
    - As well as `udp://`, a `unix://` URL may be specified if your agent can
      listen on a unix socket (e.g. the dogstatsd agent).

- `envoy_prometheus_bind_addr` - Specifies that the proxy should expose a Prometheus
  metrics endpoint to the _public_ network. It must be supplied in the form
  `ip:port` and port and the ip/port combination must be free within the network
  namespace the proxy runs. Typically the IP would be `0.0.0.0` to bind to all
  available interfaces or a pod IP address.

    -> **Note:** Envoy versions prior to 1.10 do not export timing histograms
    using the internal Prometheus endpoint.

- `envoy_stats_tags` - Specifies one or more static tags that will be added to
  all metrics produced by the proxy.

- `envoy_stats_flush_interval` - Configures Envoy's
  [`stats_flush_interval`](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-stats-flush-interval).

There are more possibilities available in the [Advanced
Configuration](#advanced-configuration) section that allow incremental or
complete control over the bootstrap configuration generated.

## Dynamic Configuration

Consul automatically generates Envoy's dynamic configuration based on its
knowledge of the cluster. Users may specify default configuration options for
each service such as which protocol they speak. Consul will use this information
to configure appropriate proxy settings for that service's proxies and also for
the upstream listeners of any downstream service.

Users can define a service's protocol in its [`service-defaults` configuration
entry](/docs/agent/config-entries/service-defaults.html). Agents with
[`enable_central_service_config`](/docs/agent/options.html#enable_central_service_config)
set to true will automatically discover the protocol when configuring a proxy
for a service. The proxy will discover the main protocol of the service it
represents and use this to configure its main public listener. It will also
discover the protocols defined for any of its upstream services and
automatically configure its upstream listeners appropriately too as below.

This automated discovery results in Consul auto-populating the `proxy.config`
and `proxy.upstreams[*].config` fields of the [proxy service
definition](/docs/connect/registration/service-registration.html) that is
actually registered.

### Proxy Config Options

These fields may also be overridden explicitly in the [proxy service
definition](/docs/connect/registration/service-registration.html), or defined in
the  [global `proxy-defaults` configuration
entry](/docs/agent/config-entries/proxy-defaults.html) to act as
defaults that are inherited by all services.


- `protocol` - The protocol the service speaks. Connect's Envoy integration
  currently supports the following `protocol` values:

  - `tcp` - Unless otherwise specified this is the default, which causes Envoy
    to proxy at L4. This provides all the security benefits of Connect's mTLS
    and works for any TCP-based protocol. Load-balancing and metrics are
    available at the connection level.
  - `http` - This specifies that the service speaks HTTP/1.x. Envoy will setup an
    `http_connection_manager` and will be able to load-balance requests
    individually to available upstream services. Envoy will also emit L7 metrics
    such as request rates broken down by HTTP response code family (2xx, 4xx, 5xx,
    etc).
  - `http2` - This specifies that the service speaks http2 (specifically h2c since
    Envoy will still only connect to the local service instance via plain TCP not
    TLS). This behaves much like `http` with L7 load-balancing and metrics but has
    additional settings that correctly enable end-to-end http2.
  - `grpc` - gRPC is a common RPC protocol based on http2. In addition to the
    http2 support above, Envoy listeners will be configured with a
    [gRPC bridge
    filter](https://www.envoyproxy.io/docs/envoy/v1.10.0/configuration/http_filters/grpc_http1_bridge_filter#config-http-filters-grpc-bridge)
    that translates HTTP/1.1 calls into gRPC, and instruments
    metrics with `gRPC-status` trailer codes.

    ~> **Note:** The protocol of a service should ideally be configured via the
    [`protocol`](/docs/agent/config-entries/service-defaults.html#protocol)
    field of a
    [`service-defaults`](/docs/agent/config-entries/service-defaults.html)
    config entry for the service. Configuring it in a
    proxy config will not fully enable some [L7
    features](/docs/connect/l7-traffic-management.html).
    It is supported here for backwards compatibility with Consul versions prior to 1.6.0.

- `bind_address` - Override the address Envoy's public listener binds to. By
  default Envoy will bind to the service address or 0.0.0.0 if there is not explicit address on the service registration.

- `bind_port` - Override the port Envoy's public listener binds to. By default
  Envoy will bind to the service port.

- `local_connect_timeout_ms` - The number of milliseconds allowed to make
  connections to the local application instance before timing out. Defaults to 5000
  (5 seconds).

### Proxy Upstream Config Options

The following configuration items may be overridden directly in the
`proxy.upstreams[].config` field of a [proxy service
definition](/docs/connect/registration/service-registration.html) or
[`sidecar_service`](/docs/connect/registration/sidecar-service.html) block.

- `protocol` - Same as above in main config but affects the listener setup for
  the upstream.

    ~> **Note:** The protocol of a service should ideally be configured via the
    [`protocol`](/docs/agent/config-entries/service-defaults.html#protocol)
    field of a
    [`service-defaults`](/docs/agent/config-entries/service-defaults.html)
    config entry for the upstream destination service. Configuring it in a
    proxy upstream config will not fully enable some [L7
    features](/docs/connect/l7-traffic-management.html).
    It is supported here for backwards compatibility with Consul versions prior to 1.6.0.

- `connect_timeout_ms` - The number of milliseconds to allow when making upstream
  connections before timing out. Defaults to 5000
  (5 seconds).

    ~> **Note:** The connection timeout for a service should ideally be
    configured via the
    [`connect_timeout`](/docs/agent/config-entries/service-resolver.html#connecttimeout)
    field of a
    [`service-resolver`](/docs/agent/config-entries/service-resolver.html)
    config entry for the upstream destination service. Configuring it in a
    proxy upstream config will override any values defined in config entries.
    It is supported here for backwards compatibility with Consul versions prior to 1.6.0.

### Mesh Gateway Options

These fields may also be overridden explicitly in the [proxy service
definition](/docs/connect/registration/service-registration.html), or defined in
the  [global `proxy-defaults` configuration
entry](/docs/agent/config_entries.html#proxy-defaults-proxy-defaults) to act as
defaults that are inherited by all services.

- `connect_timeout_ms` - The number of milliseconds to allow when making upstream
  connections before timing out. Defaults to 5000
  (5 seconds).

- `envoy_mesh_gateway_bind_tagged_addresses` - Indicates that the mesh gateway
  services tagged addresses should be bound to listeners in addition to the
  default listener address.

- `envoy_mesh_gateway_bind_addresses` - A map of additional addresses to be bound.
  This map's keys are the name of the listeners to be created and the values are
  a map with two keys, address and port, that combined make the address to bind the
  listener to. These are bound in addition to the default address.

- `envoy_mesh_gateway_no_default_bind` - Prevents binding to the default address
  of the mesh gateway service. This should be used with one of the other options
  to configure the gateways bind addresses.

## Advanced Configuration

To support more flexibility when configuring Envoy, several "lower-level" options exist
that require knowledge of Envoy's configuration format.
Many options allow configuring a subsection of either the bootstrap or
dynamic configuration using your own custom protobuf config.

We separate these into two sets, [Advanced Bootstrap
Options](#advanced-bootstrap-options) and [Escape Hatch
Overrides](#escape-hatch-overrides). Both require writing Envoy config in the
protobuf JSON encoding. Advanced options cover smaller chunks that might
commonly need to be set for tasks like configuring tracing. In contrast, escape hatches
give almost complete control over the proxy setup, but require operators to
manually code the entire configuration in protobuf JSON.

~> **Advanced Topic!** This section covers options that allow users to take almost
complete control of Envoy's configuration. We provide these options so users can
experiment or take advantage of features not yet fully supported in Consul Connect. We
plan to retain this ability in the future, but it should still be considered
experimental because it requires in-depth knowledge of Envoy's configuration format.
Users should consider Envoy version compatibility when using these features because they can configure Envoy in ways that
are outside of Consul's control. Incorrect configuration could prevent all
proxies in your mesh from functioning correctly, or bypass the security
guarantees Connect is designed to enforce.

### Configuration Formatting

All configurations are specified as strings containing the serialized proto3 JSON encoding
of the specified Envoy configuration type. They are full JSON types except where
noted.

The JSON supplied may describe a protobuf `types.Any` message with an `@type`
field set to the appropriate type (for example
`type.googleapis.com/envoy.api.v2.Listener`), or it may be the direct encoding
with no `@type` field.

### Advanced Bootstrap Options

Users may add the following configuration items to the [global `proxy-defaults`
configuration
entry](/docs/agent/config-entries/proxy-defaults.html) or
override them directly in the `proxy.config` field of a [proxy service
definition](/docs/connect/registration/service-registration.html) or
[`sidecar_service`](/docs/connect/registration/sidecar-service.html) block.

- `envoy_extra_static_clusters_json` - Specifies one or more [Envoy
  clusters](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/api/v2/cds.proto#cluster)
  that will be appended to the array of [static
  clusters](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-staticresources-clusters)
  in the bootstrap config. This allows adding custom clusters for tracing sinks
  for example. For a single cluster just encode a single object, for multiple,
  they should be comma separated with no trailing comma suitable for
  interpolating directly into a JSON array inside the braces.
- `envoy_extra_static_listeners_json` - Similar to
  `envoy_extra_static_clusters_json` but appends [static
  listener](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-staticresources-listeners) definitions.
  Can be used to setup limited access that bypasses Connect mTLS or
  authorization for health checks or metrics.
- `envoy_extra_stats_sinks_json` - Similar to `envoy_extra_static_clusters_json`
  but for [stats sinks](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-stats-sinks). These are appended to any sinks defined by use of the
  higher-level [`envoy_statsd_url`](#envoy_statsd_url) or
  [`envoy_dogstatsd_url`](#envoy_dogstatsd_url) config options.
- `envoy_stats_config_json` - The entire [stats
  config](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-stats-config).
  If provided this will override the higher-level
  [`envoy_stats_tags`](#envoy_stats_tags). It allows full control over dynamic
  tag replacements etc.
- `envoy_tracing_json` - The entire [tracing
  config](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-tracing).
  Most tracing providers will also require adding static clusters to define the
  endpoints to send tracing data to.

### Escape-Hatch Overrides

Users may add the following configuration items to the [global `proxy-defaults`
configuration
entry](/docs/agent/config-entries/proxy-defaults.html) or
override them directly in the `proxy.config` field of a [proxy service
definition](/docs/connect/registration/service-registration.html) or
[`sidecar_service`](/docs/connect/registration/sidecar-service.html) block.

- `envoy_bootstrap_json_tpl` - Specifies a template in Go template syntax that
  is used in place of [the default
  template](https://github.com/hashicorp/consul/blob/b64bda880843afaaf44591c3200f921626716849/command/connect/envoy/bootstrap_tpl.go#L87)
  when generating bootstrap via [`consul connect envoy`
  command](/docs/commands/connect/envoy.html). The variables that are available
  to be interpolated are [documented
  here](https://github.com/hashicorp/consul/blob/b64bda880843afaaf44591c3200f921626716849/command/connect/envoy/bootstrap_tpl.go#L5).
  This offers complete control of the proxy's bootstrap although major
  deviations from the default template may break Consul's ability to correctly
  manage the proxy or enforce it's security model.
- `envoy_public_listener_json` - Specifies a complete
  [Listener](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/api/v2/lds.proto)
  to be delivered in place of the main public listener that the proxy used to
  accept inbound connections. This will be used verbatim with the following
  exceptions:
  - Every `FilterChain` added to the listener will have its `TlsContext`
    overridden by the Connect TLS certificates and validation context. This
    means there is no way to override Connect's mutual TLS for the public
    listener.
  - Every `FilterChain` will have the `envoy.ext_authz` filter prepended to the
    filters array to ensure that all inbound connections are authorized by
    Connect.
- `envoy_local_cluster_json` - Specifies a complete [Envoy
  cluster](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/api/v2/cds.proto#cluster)
  to be delivered in place of the local application cluster. This allows
  customization of timeouts, rate limits, load balancing strategy etc.


The following configuration items may be overridden directly in the
`proxy.upstreams[].config` field of a [proxy service
definition](/docs/connect/registration/service-registration.html) or
[`sidecar_service`](/docs/connect/registration/sidecar-service.html) block.

~> **Note:** - When a
[`service-router`](/docs/agent/config-entries/service-router.html),
[`service-splitter`](/docs/agent/config-entries/service-splitter.html), or
[`service-resolver`](/docs/agent/config-entries/service-resolver.html) config
entry exists for a service the below escape hatches are ignored and will log a
warning.

- `envoy_listener_json` - Specifies a complete
  [Listener](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/api/v2/lds.proto)
  to be delivered in place of the upstream listener that the proxy exposes to
  the application for outbound connections. This will be used verbatim with the
  following exceptions:
  - Every `FilterChain` added to the listener will have its `TlsContext`
    overridden by the Connect TLS certificates and validation context. This
    means there is no way to override Connect's mutual TLS for the public
    listener.
- `envoy_cluster_json` - Specifies a complete [Envoy
  cluster](https://www.envoyproxy.io/docs/envoy/v1.10.0/api-v2/api/v2/cds.proto#cluster)
  to be delivered in place of the discovered upstream cluster. This allows
  customization of timeouts, circuit breaking, rate limits, load balancing
  strategy etc.
