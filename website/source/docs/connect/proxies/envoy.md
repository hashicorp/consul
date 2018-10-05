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