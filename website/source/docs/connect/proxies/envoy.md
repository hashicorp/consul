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

Envoy proxies require two types of configuration: and initial bootstrap
configuration file and dynamic configuration that it learns using
it's service discovery API.

The bootstrap configuration at a minimum needs to configure the proxy with an
identity (node id) and the location of it's local Consul agent from which it
discovers all of it's dynamic configuration. See #bootstrap-configuration for
more details.

The dynamic configuration Consul Connect provides to each Envoy includes:
 - TLS certificates and keys to enable mutual auth and keep certificates
   rotating,
 - Service discovery results for upstreams to enable the sidecar to loadbalance
   outgoing connections
 - L7 configuration including timeouts and protocol specific options.

For more information on the parts of the runtime configuration of Envoy proxies
that are currently controllable via Consul Connect see #dynamic-configuration.

While we plan to continue to enable more and more of Envoy's features through
Connect's first-class configuration over time, we recognize that some advanced
users will need more control to configure Envoy in specific ways. To enable
this, we provide several "escape hatch" options that allow low-level raw Envoy
config syntax to be provided for some sub-components in each Envoy instance
which allows full control although with operator taking responsibility for
correct configuration and Envoy version support etc.

## Bootstrap Configuration

Envoy requires an initial bootstrap configuration that directs it to the local
agent for further configuration discovery.

To assist in generating this, Consul 1.3.0 adds a [`consul connect envoy`
command](/docs/commands/connect/envoy.html). The command can either output the
bootstrap configuration directly or can generate it and then `exec` the Envoy
binary as a convenience wrapper.

Some Envoy configuration options like metrics and tracing sinks can only be
specified via the bootstrap config and so Consul Connect as of 1.5.0 adds the
ability to control some parts of the bootstrap config via service registration
options or [centralized configuration](#TODO).

The following configuration items may optionally be added to the
[Proxy.Config](#TODO) map in a proxy service registration (or sidecar_service
block).

- `envoy_statsd_url` - A URL in the form `udp://ip:port` identifying a UDP
  statsd listener that metrics should be delivered to. For example this may be
  `udp://127.0.0.1:8125` if every host has a local statsd listener. In this
  case it is convenient to configure this property once in the [global proxy
  defaults](#TODO) config entry. Currently, TCP is not supported.

    -> **Note:** currently the url **must use an ip address** not a dns name due
    to the way Envoy is setup for statsd. Later TCP support for statsd may
    alleviate this.

    The whole parameter may also be specified in the form `$ENV_VAR_NAME` which
    will cause the `consul connect envoy` command to resolve the actual URL from
    the named environment variable when it runs. This for example allows each pod
    in a Kubernetes cluster to learn of a pod-specific IP address for statsd when
    the envoy instance is bootstrapped while still allowing global config of all
    proxies to use statsd in the [global proxy defaults](#TODO) config entry. The
    env variable must contain a full valid URL value as specified above and
    nothing else. It is not possible to use ENV variables as only part of the URL
    currently.

- `envoy_dogstatsd_url` - The same as `envoy_statsd_url` with the following
  differences in behavior:
    - Envoy will use dogstatsd tags instead of statsd dot-separated metric names
    - As well as `udp://`, a `unix://` URL may be specified if your agent can
      listen on a unix socket (e.g. the dogstatsd agent).

- `prometheus_bind_addr` - Specifies that the proxy should expose a Prometheus
  metrics endpoint to the _public_ network. It must be supplied in the form
  `ip:port` and port and the ip/port combination must be free within the network
  namespace the proxy runs. Typically the IP would be `0.0.0.0`


## Limitations

The following list limitations of the Envoy integration as released in 1.3.0.
All of these are planned to be lifted in the near future.

 * Default Envoy configuration only supports Layer 4 (TCP) proxying. More
   [advanced listener configuration](#advanced-listener-configuration) is
   possible but experimental and requires deep Envoy knowledge. First class
   workflows for configuring Layer 7 features across the cluster are planned for
   the near future.
 * There is currently no way to override the configuration of upstream clusters
   which makes it impossible to configure Envoy features like circuit breakers,
   load balancing policy, custom protocol settings etc. This will be fixed in a
   near-future release first with an "escape hatch" similar to the one for
   listeners below, then later with first-class support.
 * The configuration delivered to Envoy is suitable for a sidecar proxy
   currently. Later we plan to support more flexibility to be able to configure
   Envoy as an edge router or gateway and similar.
 * There is currently no way to disable the public listener and have a "client
   only" sidecar for services that don't expose Connect-enabled service but want
   to consume others. This will be fixed in a near-future release.
 * Once authorized, a persistent TCP connection will not be closed if the
   intentions change to deny access. This is currently a limitation of how TCP
   proxy and network authz filter work in Envoy. All new connections will be
   denied though and destination services can limit exposure by closing inbound
   connections periodically or by a rolling restart of the destination service
   as an emergency measure.

## Bootstrap Configuration

Envoy requires an initial bootstrap configuration that directs it to the local
agent for further configuration discovery.

To assist in generating this, Consul 1.3.0 adds a [`consul connect envoy`
command](/docs/commands/connect/envoy.html). The command can either output the
bootstrap configuration directly or can generate it and then `exec` the Envoy
binary as a convenience wrapper.

Some Envoy configuration options like metrics and tracing sinks can only be
specified via the bootstrap config currently and so a custom bootstrap must be
used. In order to work with Connect it's necessary to start with the following
basic template and add additional configuration as needed.

```yaml
admin:
  # access_log_path and address are required by Envoy, Consul doesn't care what
  # they are set to though and never accesses the admin API.
node:
  # cluter is required by Envoy but Consul doesn't use it
  cluster: "<cluster_name"
  # id must be the ID (not name if they differ) of the proxy service
  # registration in Consul
  id: "<proxy_service_id>"
static_resources:
  clusters:
  # local_agent is the "cluster" used to make further discovery requests for 
  # config and should point to the gRPC port of the local Consul agent instance.
  - name: local_agent
    connect_timeout: 1s
    type: STATIC
    # tls_context is needed if and only if Consul agent TLS is configured
    tls_context:
      common_tls_context:
        validation_context:
          trusted_ca:
            filename: "<path to CA cert file Consul is using>"
    http2_protocol_options: {}
    hosts:
    - socket_address:
       address: "<agent's local IP address, usually 127.0.0.1>"
       port_value: "<agent's grpc port, default 8502>"
dynamic_resources:
  lds_config:
    ads: {}
  cds_config:
    ads: {}
  ads_config:
    api_type: GRPC
    grpc_services:
      initial_metadata:
      - key: "x-consul-token"
        token: "<Consul ACL token with service:write on the target service>"
      envoy_grpc:
        cluster_name: local_agent
```

This configures a "cluster" pointing to the local Consul agent and sets that as
the target for discovering all types of dynamic resources.

~> **Security Note**: The bootstrap configuration must contain the Consul ACL
token authorizing the proxy to identify as the target service. As such it should
be treated as a secret value and handled with care - an attacker with access to
one is able to obtain Connect TLS certificates for the target service and so
access anything that service is authorized to connect to.

## Advanced Listener Configuration

Consul 1.3.0 includes initial Envoy support which includes automatic Layer 4
(TCP) proxying over mTLS, and authorization. Near future versions of Consul will
bring Layer 7 features like HTTP-path-based routing, retries, tracing and more.

-> **Advanced Topic!** This section covers an optional way of taking almost
complete control of Envoy's listener configuration which is provided as a way to
experiment with advanced integrations ahead of full layer 7 feature support.
While we don't plan to remove the ability to do this in the future, it should be
considered experimental and requires in-depth knowledge of Envoy's configuration
format.

For advanced users there is an "escape hatch" available in 1.3.0. The
`proxy.config` map in the [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) may contain a
special key called `envoy_public_listener_json`. If this is set, it's value must
be a string containing the serialized proto3 JSON encoding of a complete [envoy
listener
config](https://www.envoyproxy.io/docs/envoy/v1.8.0/api-v2/api/v2/lds.proto).
Each upstream listener may also be customized in the same way by adding a
`envoy_listener_json` key to the `config` map of [the upstream
definition](/docs/connect/proxies.html#upstream-configuration-reference).

The JSON supplied may describe a protobuf `types.Any` message with `@type` set
to `type.googleapis.com/envoy.api.v2.Listener`, or it may be the direct encoding
of the listener with no `@type` field.

Once parsed, it is passed to Envoy in place of the listener config that Consul
would typically configure. The only modifications Consul will make to the config
provided are noted below.

#### Public Listener Configuration

For the `proxy.config.envoy_public_listener_json`, every `FilterChain` added to
the listener will have it's `TlsContext` overwritten with the Connect TLS
certificates. This means there is no way to override Connect TLS settings or the
requirement for all inbound clients to present valid Connect certificates.

Also, every `FilterChain` will have the `envoy.ext_authz` filter prepended to
the filters array to ensure that all incoming connections must be authorized
explicitly by the Consul agent based on their presented client certificate.

To work properly with Consul Connect, the public listener should bind to the
same address in the service definition so it is discoverable. It may also use
the special cluster name `local_app` to forward requests to a single local
instance if the proxy was configured [as a
sidecar](/docs/connect/proxies.html#sidecar-proxy-fields).

#### Example

The following example shows a public listener being configured with an http
connection manager. As specified this behaves exactly like the default TCP proxy
filter however it provides metrics on HTTP request volume and response codes.

If additional config outside of the listener is needed (for example the
top-level `tracing` configuration to send traces to a collecting service), those
currently need to be added to a custom bootstrap. You may generate the default
connect bootstrap with the [`consul connect envoy -bootstrap`
command](/docs/commands/connect/envoy.html) and then add the required additional
resources.

```text
service {
  kind = "connect-proxy"
  name = "web-http-aware-proxy"
  port = 8080
  proxy {
    destination_service_name = "web"
    destination_service_id = "web"
    config {
      envoy_public_listener_json = <<EOL
        {
          "@type": "type.googleapis.com/envoy.api.v2.Listener",
          "name": "public_listener:0.0.0.0:18080",
          "address": {
            "socketAddress": {
              "address": "0.0.0.0",
              "portValue": 8080
            }
          },
          "filterChains": [
            {
              "filters": [
                {
                  "name": "envoy.http_connection_manager",
                  "config": {
                    "stat_prefix": "public_listener",
                    "route_config": {
                      "name": "local_route",
                      "virtual_hosts": [
                        {
                          "name": "backend",
                          "domains": ["*"],
                          "routes": [
                            {
                              "match": {
                                "prefix": "/"
                              },
                              "route": {
                                "cluster": "local_app"
                              }
                            }
                          ]
                        }
                      ]
                    },
                    "http_filters": [
                      {
                        "name": "envoy.router",
                        "config": {}
                      }
                    ]
                  }
                }
              ]
            }
          ]
        }
        EOL
    }
  }
}
```

#### Upstream Listener Configuration

For the upstream listeners `proxy.upstreams[].config.envoy_listener_json`, no
modification is performed. The `Clusters` served via the xDS API all have the
correct client certificates and verification contexts configured so outbound
traffic should be authenticated.

Each upstream may separately choose to define a custom listener config. If
multiple upstreams define them care must be taken to ensure they all listen on
separate ports.

Currently there is no way to disable a listener for an upstream, or modify how
upstream service discovery clusters are delivered. Richer support for features
like this is planned for the near future.

