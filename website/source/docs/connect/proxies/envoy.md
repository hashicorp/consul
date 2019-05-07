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
API](https://github.com/envoyproxy/data-plane-api/blob/master/XDS_PROTOCOL.md).

Consul can configure Envoy sidecars to proxy http/1.1, http2 or gRPC traffic at
L7 or any other tcp-based protocol at L4. Prior to Consul 1.5.0 Envoy proxies
could only proxy tcp at L4.

Currently configuration of additional L7 features is limited, however we have
plans to support a wide range of further features in the next major release
cycle.

As an interim solution, [custom Envoy configuration](#custom-configuration) can
be specified in [proxy service definition](/docs/connect/proxies.html) allowing
more powerful features of Envoy to be used.

## Supported Versions

Consul's Envoy support was added in version 1.3.0. The following table shows
compatible Envoy versions.

| Consul Version | Compatible Envoy Versions |
|---|---|
| 1.5.x and higher | 1.9.1, 1.8.0† |
| 1.3.x, 1.4.x | 1.9.1, 1.8.0†, 1.7.0† |

 ~> **† Security Note:** Envoy versions lower than 1.9.1 are vulnerable to
 [CVE-2019-9900](https://github.com/envoyproxy/envoy/issues/6434) and
 [CVE-2019-9901](https://github.com/envoyproxy/envoy/issues/6435). Both are
 related to HTTP request parsing and so only affect Consul Connect users if they
 have configured HTTP routing rules via the ["escape
 hatch"](#custom-configuration). Still, we recommend Envoy 1.9.1 be used where
 possible.

## Getting Started

To get started with Envoy and see a working example you can follow the [Using
Envoy with Connect](/docs/guides/connect-envoy.html) guide.

## Configuration

Envoy proxies require two types of configuration: an initial _bootstrap
configuration_ and dynamic configuration that is discovered from a "management
server", in this case Consul.

The bootstrap configuration at a minimum needs to configure the proxy with an
identity (node id) and the location of it's local Consul agent from which it
discovers all of it's dynamic configuration. See [Bootstrap
Configuration](#bootstrap-configuration) for more details.

The dynamic configuration Consul Connect provides to each Envoy includes:

 - TLS certificates and keys to enable mutual auth and keep certificates
   rotating.
 - Service discovery results for upstreams to enable the sidecar to load-balance
   outgoing connections.
 - L7 configuration including timeouts and protocol specific options.

For more information on the parts of the runtime configuration of Envoy proxies
that are currently controllable via Consul Connect see [Dynamic
Configuration](#dynamic-configuration).

We plan to continue to enable more and more of Envoy's features through
Connect's first-class configuration over time, however some advanced users will
need more control to configure Envoy in specific ways. To enable this, we
provide several ["escape hatch"](#advanced-configuration) options that allow
low-level raw Envoy config syntax to be provided for some sub-components in each
Envoy instance which allows full control although with operator taking
responsibility for correct configuration and Envoy version support etc.

## Bootstrap Configuration

Envoy requires an initial bootstrap configuration file. The easiest way to
create this is the [`consul connect envoy`
command](/docs/commands/connect/envoy.html). The command can either output the
bootstrap configuration directly to stdout or can generate it and then `exec`
the Envoy binary as a convenience wrapper.

Some Envoy configuration options like metrics and tracing sinks can only be
specified via the bootstrap configuration and so Connect as of Consul 1.5.0 adds
the ability to control some parts of the bootstrap config via proxy
configuration options.

The following configuration items may be added to the [global `proxy-defaults`
configuration entry](#TODO) or overridden directly in the `proxy.config` field
of a [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) or
[`sidecar_service`](/docs/connect/proxies/sidecar-service.html) block.

- `envoy_statsd_url` - A URL in the form `udp://ip:port` identifying a UDP
  statsd listener that metrics should be delivered to. For example this may be
  `udp://127.0.0.1:8125` if every host has a local statsd listener. In this
  case it is convenient to configure this property once in the [global proxy
  defaults](#TODO) config entry. Currently, TCP is not supported.

    -> **Note:** currently the url **must use an ip address** not a dns name due
    to the way Envoy is setup for statsd.

    The whole parameter may also be specified in the form `$ENV_VAR_NAME` which
    will cause the `consul connect envoy` command to resolve the actual URL from
    the named environment variable when it runs. This for example allows each
    pod in a Kubernetes cluster to learn of a pod-specific IP address for statsd
    when the envoy instance is bootstrapped while still allowing global
    configuration of all proxies to use statsd in the [global `proxy-defaults`
    configuration entry](#TODO). The env variable must contain a full valid URL
    value as specified above and nothing else. It is not possible to use
    environment variables as only part of the URL currently.

- `envoy_dogstatsd_url` - The same as `envoy_statsd_url` with the following
  differences in behavior:
    - Envoy will use dogstatsd tags instead of statsd dot-separated metric names
    - As well as `udp://`, a `unix://` URL may be specified if your agent can
      listen on a unix socket (e.g. the dogstatsd agent).

- `envoy_prometheus_bind_addr` - Specifies that the proxy should expose a Prometheus
  metrics endpoint to the _public_ network. It must be supplied in the form
  `ip:port` and port and the ip/port combination must be free within the network
  namespace the proxy runs. Typically the IP would be `0.0.0.0` to bind to all
  available interfaces or a pod IP address.

    -> **Note:** Envoy versions prior to 1.10 do not export timing histograms
    using the internal prometheus endpoint. Consul 1.5.0 [doesn't yet support
    Envoy 1.10](#supported-versions) although support will soon be added.

- `envoy_stats_tags` - Specifies one or more static tags that will be added to
  all metrics produced by the proxy.

- `envoy_stats_flush_interval` - Configures Envoy's
  [`stats_flush_interval`](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-stats-flush-interval).

There are more possibilities available in the [Advanced
Configuration](#advanced-configuration) section that allow incremental or
complete control over the bootstrap configuration generated.

## Dynamic Configuration

Envoy's dynamic configuration is generated by Consul and is automatic. For Envoy
we generate different listener configurations depending on the service's
protocol which is defined using [`service-defaults` configuration
entries](#TODO).

The following configuration items may be added to the [global `proxy-defaults`
configuration entry](#TODO) or overridden directly in the `proxy.config` field
of a [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) or
[`sidecar_service`](/docs/connect/proxies/sidecar-service.html) block.

- `protocol` - The protocol the service speaks. The following `protocol` values
  are supported by Connect's Envoy integration currently:
  - `tcp` - The default is plain TCP and will proxy at L4. This gets all the
    security benefits of Connect's mTLS and works for any TCP-based protocol.
    Load-balancing and metrics are available at the connection level.
  - `http` - This specified the service speaks HTTP/1.x. Envoy will setup an
    `http_connection_manager` and will be able to load-balance requests
    individually to available upstream services. Envoy will also emit L7 metrics
    such as request rates broken down by HTTP response code family (2xx, 4xx, 5xx,
    etc).
  - `http2` - This specifies that the service speaks http2. Specifically h2c since
    Envoy will still only connect to the local service instance via plain TCP not
    TLS. This behaves much like `http` with L7 load-balancing and metrics but has
    additional settings that correctly enable end-to-end http2. Envoy in the
    future may support automatic upgrades between HTTP/1.x and http2 and not need
    explicit configuration.
  - `grpc` - gRPC is a common RPC protocol based on http2. In addition to the
    http2 support above, a service with `grpc` protocol will be configured with a
    [gRPC bridge
    filter](https://www.envoyproxy.io/docs/envoy/v1.9.1/configuration/http_filters/grpc_http1_bridge_filter#config-http-filters-grpc-bridge)
    that allows HTTP/1.1 calls to be translated into gRPC as well as instrumenting
    metrics with `gRPC-status` trailer codes.
- `local_connect_timeout_ms` - The number of milliseconds to allow when making
  connections the local application instance before timing out. Defaults to 5000
  (5 seconds).

The following configuration items may be overridden directly in the
`proxy.upstreams[].config` field of a [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) or
[`sidecar_service`](/docs/connect/proxies/sidecar-service.html) block.

- `protocol` - Same as above in main config but affects the listener setup for
  the upstream.
- `connect_timeout_ms` - The number of milliseconds to allow when making upstream
  connections before timing out. Defaults to 5000
  (5 seconds).

## Advanced Configuration

To support more flexibility when configuring Envoy, several options exist that
are "lower level" and require a good knowledge of Envoy's configuration format
and options. Many require configuring a subsection of either the bootstrap or
dynamic configuration using your own custom protobuf config.

We separate these into two sets, [Advanced Bootstrap
Options](#advanced-bootstrap-options) and [Escape Hatch
Overrides](#escape-hatch-overrides). Both require writing Envoy config in it's
protobuf JSON encoding, however advanced options are smaller chunks that might
commonly need to be set for tasks like configuring tracing, while escape hatches
give almost complete control over the proxy setup at the expense of needing to
manually code the entire configuration in protobuf JSON.

-> **Advanced Topic!** This section covers optional ways of taking almost
complete control of Envoy's configuration which is provided as a way to
experiment or take advantage of features not yet fully supported. While we don't
plan to remove the ability to do this in the future, it should be considered
experimental and requires in-depth knowledge of Envoy's configuration format.
Envoy version compatibility when using these features should be considered and
is outside of Consul's control. Incorrect configuration could prevent all
proxies in your mesh from functioning correctly or bypass the security
guarantees Connect is designed to enforce.

### Configuration Formatting

They are all specified as strings containing the serialized proto3 JSON encoding
of the specified Envoy configuration type. They are full JSON types except where
noted.

The JSON supplied may describe a protobuf `types.Any` message with an `@type`
field set to the appropriate type (for example
`type.googleapis.com/envoy.api.v2.Listener`), or it may be the direct encoding
with no `@type` field.

### Advanced Bootstrap Options

The following configuration items may be added to the [global `proxy-defaults`
configuration entry](#TODO) or overridden directly in the `proxy.config` field
of a [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) or
[`sidecar_service`](/docs/connect/proxies/sidecar-service.html) block.

- `envoy_extra_static_clusters_json` - Specifies one or more [Envoy
  clusters](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/api/v2/cds.proto#cluster)
  that will be appended to the array of [static
  clusters](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-staticresources-clusters)
  in the bootstrap config. This allows adding custom clusters for tracing sinks
  for example. For a single cluster just encode a single object, for multiple,
  they should be comma separated with no trailing comma suitable for
  interpolating directly into a JSON array inside the braces.
- `envoy_extra_static_listeners_json` - As with
  `envoy_extra_static_clusters_json` but appends [static
  listener](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-staticresources-listeners) definitions.
  Can be used to setup limited access that bypasses Connect mTLS or
  authorization for health checks or metrics.
- `envoy_extra_stats_sinks_json` - As with `envoy_extra_static_clusters_json`
  but for [stats sinks](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-stats-sinks). These are appended to any sinks defined by use of the
  higher-level [`envoy_statsd_url`](#envoy_statsd_url) or
  [`envoy_dogstatsd_url`](#envoy_dogstatsd_url) config options.
- `envoy_stats_config_json` - The entire [stats
  config](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-stats-config).
  If provided this will override the higher-level
  [`envoy_stats_tags`](#envoy_stats_tags). It allows full control over dynamic
  tag replacements etc.
- `envoy_tracing_json` - The entire [tracing
  config](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/config/bootstrap/v2/bootstrap.proto#envoy-api-field-config-bootstrap-v2-bootstrap-tracing).
  Most tracing providers will also require adding static clusters to define the
  endpoints to send tracing data to.

### Escape-Hatch Overrides

The following configuration items may be added to the [global `proxy-defaults`
configuration entry](#TODO) or overridden directly in the `proxy.config` field
of a [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) or
[`sidecar_service`](/docs/connect/proxies/sidecar-service.html) block.

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
  [Listener](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/api/v2/lds.proto)
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
  cluster](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/api/v2/cds.proto#cluster)
  to be delivered in place of the local application cluster. This allows
  customization of timeouts, rate limits, load balancing strategy etc.


The following configuration items may be overridden directly in the
`proxy.upstreams[].config` field of a [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) or
[`sidecar_service`](/docs/connect/proxies/sidecar-service.html) block.

- `envoy_listener_json` - Specifies a complete
  [Listener](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/api/v2/lds.proto)
  to be delivered in place of the upstream listener that the proxy exposes to
  the application for outbound connections. This will be used verbatim with the
  following exceptions:
  - Every `FilterChain` added to the listener will have its `TlsContext`
    overridden by the Connect TLS certificates and validation context. This
    means there is no way to override Connect's mutual TLS for the public
    listener.
- `envoy_cluster_json` - Specifies a complete [Envoy
  cluster](https://www.envoyproxy.io/docs/envoy/v1.9.1/api-v2/api/v2/cds.proto#cluster)
  to be delivered in place of the discovered upstream cluster. This allows
  customization of timeouts, circuit breaking, rate limits, load balancing
  strategy etc.
