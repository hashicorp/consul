---
layout: "docs"
page_title: "Connect - Envoy Integration"
sidebar_current: "docs-connect-proxies-envoy"
description: |-
  Consul Connect has first-class support for configuring Envoy proxy.
---

# Envoy Integration

Consul Connect has first class support for using
[envoy](https://www.envoyproxy.io) as a proxy. Consul configures envoy by
optionally exposing a gRPC service on the local agent that serves [envoy's xDS
configuration
API](https://github.com/envoyproxy/data-plane-api/blob/master/XDS_PROTOCOL.md).

Currently Consul only supports TCP proxying between services, however HTTP and
gRPC features are planned for the near future along with first class ways to
configure them in Consul.

As an interim solution, [custom envoy configuration](#custom-configuration) can
be specified in [proxy service definition](/docs/connect/proxies.html) allowing
more powerful features of envoy to be used.

## Getting Started

### Installing Envoy

The simplest way to try out envoy with Consul locally is using Docker. Envoy
doesn't release binaries outside of their official Docker image. If you can
build envoy directly then the [`consul connect envoy`
command](/docs/commands/connect/envoy.html) command can be used directly on your
local machine to start envoy, however for this guide we'll use the Docker image.

While the [`consul connect envoy` command](/docs/commands/connect/envoy.html)
supports generating bootstrap config on the host that we could then mount in to
the standard envoy Docker container, it's simpler to be able to use it to run
docker directly which requires both consul and envoy binaries in one container.

Using Docker 17.05 or higher (with multi-stage builds), create a Dockerfile with
the following content:

```text
FROM consul:latest
FROM envoyproxy/envoy:v1.8.0
COPY --from=0 /bin/consul /bin/consul
ENTRYPOINT ["dumb-init", "consul", "connect", "envoy"]
```

This takes the consul binary from the latest release image and copies it into a
new image based on the official envoy build.

This can be built locally with:

```text
docker build -t consul-envoy .
```

### Agent Setup

To get a simple example working, run a single agent in `-dev` mode using the
latest Docker image. It's possible to run the agent on the Docker host but it
complicates the rest of the steps below due to networking config on different
platforms. This guide will start all containers using Docker `host` network
which is not a good simulation of a production setup, but makes the following
steps much simpler.

-> **Note:** `-dev` mode enables the gRPC server on port 8502 by default. For a
production agent you'll need to [explicitly configure the gRPC
port](/docs/agent/options.html#grpc_port).

In order to start a proxy instance, a [proxy service
definition](/docs/connect/proxies.html) must exist on your local agent. The
simplest way to create one is using the [sidecar service
registration](/docs/connect/proxies/sidecar-service.html) syntax.

Create a config file called `envoy_demo.hcl` containing the following service
definitions.

```text
services {
  name = "web"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams {
          destination_name = "db"
          local_bind_port = 9191
        }
      }
    }
  }
}
services {
  name = "db"
  port = 9090
  connect {
    sidecar_service {}
  }
}
```

Consul agent can now be started in dev mode using host networking with that
config:

```text
$ docker run --rm -d -v$(pwd)/envoy_demo.hcl:/etc/consul/envoy_demo.hcl \
  --network host consul:latest \
  agent -dev -config-file /etc/consul/envoy_demo.hcl
```

### Running Envoy

With the above setup, envoy proxies for each service instance can now be run
using:

```text
$ docker run --rm -d --network host \
  consul-envoy -sidecar-for web
3f213a3cf9b7583a194dd0507a31e0188a03fc1b6e165b7f9336b0b1bb2baccb
$ docker run --rm -d --network host \
  consul-envoy -sidecar-for db -admin-bind localhost:19001
d8399b54ee0c1f67d729bc4c8b6e624e86d63d2d9225935971bcb4534233012b
```

The `-admin-bind` flag on the second proxy command is needed because both
proxies are running on the host network and so can't bind to the same port for
their admin API (which cannot be disabled).

To see the output of these you can use `docker logs`. To see more verbose
information you can add `-- -l debug` to the end of the command above. This is
passing the `-l` (log-level) option directly through to envoy. With debug level
logs you should see the config being delivered to the proxy.

### Testing Connectivity

To test basic connectivity, run a dummy TCP service to act as the "db". This
example will use a simple TCP echo server. This is started in a container
sharing the host network namespace again to keep things simple. It listens on
port 9090 as declared in the service definition registered at the start. The db
proxy instance will attempt to connect to it on localhost:9090 by default which
will work here thanks to shared host networking.

```text
$ docker run -d --network host abrarov/tcp-echo --port 9090
```

Finally, to simulate acting as the `web` application, use a netcat container
also running in the host network namespace. The `web` service definition
declared the `db` service as an upstream dependency with a local bind port of
9191 so the web proxy should be proxying connections from localhost:9191 to the
`db` service.

```text
$ docker run -ti --rm --network host gophernet/netcat localhost 9191
Hello World!
Hello World!
^C
```

### Testing Authorization

To test that Connect is controlling authorization for the DB service, add an
explicit deny rule:

```
$ docker run -ti --rm --network host consul:latest intention create -deny web db
Created: web => db (deny)
```

Now, new connections as tested above will be denied. Depending on a few factors,
netcat may not see the connection being closed but will not get a response from
the service.

```text
$ docker run -ti --rm --network host gophernet/netcat localhost 9191
Hello?
Anyone there?
^C
```

-> **Note:** envoy will not re-authenticate already established TCP connections
so if you still have the netcat terminal open from before that will still be
able to communicate with the "db". _New_ connections should be denied though.

Removing the intention restores connectivity.

```
$ docker run -ti --rm --network host consul:latest intention delete web db
Intention deleted.
$ docker run -ti --rm --network host gophernet/netcat localhost 9191
Hello?
Hello?
^C
```

## Advanced Listener Configuration

Consul 1.3.0 includes initial envoy support which includes automatic Layer 4
(TCP) proxying over mTLS, and authorization. Near future versions of Consul will
bring Layer 7 features like HTTP-path-based routing, retries, tracing and more.

-> **Advanced Topic!** This section covers an optional way of taking almost
complete control of envoy's listener configuration which is provided as a way to
experiment with advanced integrations ahead of full layer 7 feature support.
While we don't plan to remove the ability to do this in the future, it should be
considered experimental and requires in-depth knowledge of envoy's configuration
format.

For advanced users there is an "escape hatch" available in 1.3.0 - the "opaque"
proxy configuration map in the [proxy service
definition](/docs/connect/proxies.html#proxy-service-definitions) may contain a
special key called `envoy_public_listener_json`. If this is set, it's value must
be a string containing the serialized proto3 JSON encoding of a complete [envoy
listener
config](https://www.envoyproxy.io/docs/envoy/v1.8.0/api-v2/api/v2/lds.proto).
Each upstream listener may also be customized in the same way by adding a
`envoy_listener_json` key to the `config` map of [the upstream
definition](/docs/connect/proxies.html#upstream-configuration-reference).

The JSON supplied may describe a `types.Any` message with `@type` set to
`type.googleapis.com/envoy.api.v2.Listener`, or it may be the direct encoding of
the listener with no `@type`. It must parse correctly as one of those though.

Once parsed, it is passed verbatim to Envoy in place of the listener config that
Consul would typically configure. The only modifications Consul will make to the
config provided are noted below along.

#### Public Listener Configuration

For the `proxy.config.envoy_public_listener_json`, very `FilterChain` added to
the listener will have it's `TlsContext` overwritten with the Connect TLS
certificates. For now this means there is no way to override Connect TLS
settings or the requirement for clients to present valid Connect certificates.

Also, every `FilterChain` will have the `envoy.ext_authz` filter prepended to
the filters array to ensure that all incoming connections must be authorized
explicitly by the Consul agent based on the client certificate.

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
    destination_service_name web
    destination_service_id web
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

