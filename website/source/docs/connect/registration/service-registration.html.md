---
layout: "docs"
page_title: "Connect - Service Registration"
sidebar_current: "docs-connect-registration-service-registration"
description: |-
  A per-service proxy sidecar transparently handles inbound and outbound service connections. You can register these sidecars with sane defaults by nesting their definitions in the service definition.
---

# Proxy Service Registration

To function as a Connect proxy, proxies must be declared as a proxy types in
their service definitions, and provide information about the service they
represent.

To declare a service as a proxy, the service definition must contain
the following fields:

  * `kind` `(string)` must be set to `connect-proxy`. This declares that the
    service is a proxy type.

  * `proxy.destination_service_name` `(string)` must be set to the service that
    this proxy is representing. Note that this replaces `proxy_destination` in
    versions 1.2.0 to 1.3.0.

    ~> **Deprecation Notice:** From version 1.2.0 to 1.3.0, proxy destination was
    specified using `proxy_destination` at the top level. This will continue to work
    until at least 1.5.0 but it's highly recommended to switch to using
    `proxy.destination_service_name`.

  * `port` `(int)` must be set so that other Connect services can discover the
    exact address for connections. `address` is optional if the service is being
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
traffic to. In this case the following fields should also be set if you are deploying your proxy as a sidecar but defining it in its own service registration:

  * `proxy.destination_service_id` `(string: <required>)` is set to the _id_
    (and not the _name_ if they are different) of the specific service instance
    that is being proxied. The proxied service is assumed to be registered on
    the same agent although it's not strictly validated to allow for
    un-coordinated registrations.

  * `proxy.local_service_port` `(int: <required>)` must specify the port the
    proxy should use to connect to the _local_ service instance.

  * `proxy.local_service_address` `(string: "")` can be set to override the IP or
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
    "upstreams": [],
    "mesh_gateway": {},
    "expose": {}
  },
  "port": 8181
}
```

#### Proxy Parameters

 - `destination_service_name` `(string: <required>)` - Specifies the _name_ of the
   service this instance is proxying. Both side-car and centralized
   load-balancing proxies must specify this. It is used during service
   discovery to find the correct proxy instances to route to for a given service
   name.

 - `destination_service_id` `(string: "")` - Specifies the _ID_ of a single
   specific service instance that this proxy is representing. This is only valid
   for side-car style proxies that run on the same node. It is assumed that the
   service instance is registered via the same Consul agent so the ID is unique
   and has no node qualifier. This is useful to show in tooling which proxy
   instance is a side-car for which application instance and will enable
   fine-grained analysis of the metrics coming from the proxy.

 - `local_service_address` `(string: "")` - Specifies the address a side-car
   proxy should attempt to connect to the local application instance on.
   Defaults to 127.0.0.1.

 - `local_service_port` `(int: <optional>)` - Specifies the port a side-car
   proxy should attempt to connect to the local application instance on.
   Defaults to the port advertised by the service instance identified by
   `destination_service_id` if it exists otherwise it may be empty in responses.

 - `config` `(object: {})` - Specifies opaque config JSON that will be
   stored and returned along with the service instance from future API calls.

 - `upstreams` `(array<Upstream>: [])` - Specifies the upstream services
   this proxy should create listeners for. The format is defined in
   [Upstream Configuration Reference](#upstream-configuration-reference).

 - `mesh_gateway` `(object: {})` - Specifies the mesh gateway configuration
   for this proxy. The format is defined in the [Mesh Gateway Configuration Reference](#mesh-gateway-configuration-reference).
   
 - `expose` `(object: {})` - Specifies the configuration to expose HTTP paths through this proxy. 
   The format is defined in the [Expose Paths Configuration Reference](#expose-paths-configuration-reference),
   and is only compatible with an Envoy proxy.

### Upstream Configuration Reference

The following examples show all possible upstream configuration parameters.

-> Note that `snake_case` is used here as it works in both [config file and API
registrations](/docs/agent/services.html#service-definition-parameter-case).

Upstreams support multiple destination types. Both examples are shown below
followed by documentation for each attribute.

#### Service Destination

```json
{
  "destination_type": "service",
  "destination_name": "redis",
  "datacenter": "dc1",
  "local_bind_address": "127.0.0.1",
  "local_bind_port": 1234,
  "config": {},
  "mesh_gateway": {
    "mode": "local"
  }
},
```

#### Prepared Query Destination

```json
{
  "destination_type": "prepared_query",
  "destination_name": "database",
  "local_bind_address": "127.0.0.1",
  "local_bind_port": 1234,
  "config": {}
},
```

* `destination_name` `(string: <required>)` - Specifies the name of the service
  or prepared query to route connect to. The prepared query should be the name
  or the ID of the prepared query.
* `local_bind_port` `(int: <required>)` - Specifies the port to bind a local
  listener to for the application to make outbound connections to this upstream.
* `local_bind_address` `(string: "")` - Specifies the address to bind a
  local listener to for the application to make outbound connections to this
  upstream. Defaults to `127.0.0.1`.
* `destination_type` `(string: "")` - Specifies the type of discovery
  query to use to find an instance to connect to. Valid values are `service` or
  `prepared_query`. Defaults to `service`.
* `datacenter` `(string: "")` - Specifies the datacenter to issue the
  discovery query too. Defaults to the local datacenter.
* `config` `(object: {})` - Specifies opaque configuration options that
  will be provided to the proxy instance for this specific upstream. Can contain
  any valid JSON object. This might be used to configure proxy-specific features
  like timeouts or retries for the given upstream. See the [built-in proxy
  configuration
  reference](/docs/connect/proxies/built-in.html#proxy-upstream-config-key-reference) for
  options available when using the built-in proxy. If using Envoy as a proxy,
  see [Envoy configuration
  reference](/docs/connect/proxies/envoy.html#proxy-upstream-config-options)
* `mesh_gateway` `(object: {})` - Specifies the mesh gateway configuration
   for this proxy. The format is defined in the [Mesh Gateway Configuration Reference](#mesh-gateway-configuration-reference).


### Mesh Gateway Configuration Reference

The following examples show all possible mesh gateway configurations.

-> Note that `snake_case` is used here as it works in both [config file and API
registrations](/docs/agent/services.html#service-definition-parameter-case).

#### Using a Local/Egress Gateway in the Local Datacenter

```json
{
  "mode": "local"
}
```

#### Direct to a Remote/Ingress in a Remote Datacenter
```json
{
  "mode": "remote"
}
```

#### Prevent Using a Mesh Gateway
```json
{
  "mode": "none"
}
```

#### Default Mesh Gateway Mode
```json
{
  "mode": ""
}
```

* `mode` `(string: "")` - This defines the mode of operation for how
  upstreams with a remote destination datacenter get resolved.
  - `"local"` - Mesh gateway services in the local datacenter will be used
     as the next-hop destination for the upstream connection.
  - `"remote"` - Mesh gateway services in the remote/target datacenter will
     be used as the next-hop destination for the upstream connection.
  - `"none"` - No mesh gateway services will be used and the next-hop destination
     for the connection will be directly to the final service(s).
  - `""` - Default mode. The default mode will be `"none"` if no other configuration
     enables them. The order of precedence for setting the mode is
       1. Upstream
       2. Proxy Service's `Proxy` configuration
       3. The `service-defaults` configuration for the service.
       4. The `global` `proxy-defaults`.

### Expose Paths Configuration Reference

The following examples show possible configurations to expose HTTP paths through Envoy.

Exposing paths through Envoy enables a service to protect itself by only listening on localhost, while still allowing 
non-Connect-enabled applications to contact an HTTP endpoint. 
Some examples include: exposing a `/metrics` path for Prometheus or `/healthz` for kubelet liveness checks.

-> Note that `snake_case` is used here as it works in both [config file and API
registrations](/docs/agent/services.html#service-definition-parameter-case).

#### Expose listeners in Envoy for HTTP and GRPC checks registered with the local Consul agent

```json
{
  "expose": {
    "checks": true
  } 
}
```

#### Expose an HTTP listener in Envoy at port 21500 that routes to an HTTP server listening at port 8080

```json
{
  "expose": {
    "paths": [
      {
        "path": "/healthz",
        "local_path_port": 8080,
        "listener_port": 21500
      }
    ]
  } 
}
```

#### Expose an HTTP2 listener in Envoy at port 21501 that routes to a gRPC server listening at port 9090

```json
{
  "expose": {
    "paths": [
      {
        "path": "/grpc.health.v1.Health/Check",
        "protocol": "http2",
        "local_path_port": 9090,
        "listener_port": 21501
      }
    ]
  } 
}
```

* `checks` `(bool: false)` - If enabled, all HTTP and gRPC checks registered with the agent are exposed through Envoy.
  Envoy will expose listeners for these checks and will only accept connections originating from localhost or Consul's 
  [advertise address](/docs/agent/options.html#advertise). The port for these listeners are dynamically allocated from 
  [expose_min_port](/docs/agent/options.html#expose_min_port) to [expose_max_port](/docs/agent/options.html#expose_max_port). 
  This flag is useful when a Consul client cannot reach registered services over localhost. One example is when running 
  Consul on Kubernetes, and Consul agents run in their own pods.
* `paths` `array<Path>: []` - A list of paths to expose through Envoy.
  - `path` `(string: "")` - The HTTP path to expose. The path must be prefixed by a slash. ie: `/metrics`.
  - `local_path_port` `(int: 0)` - The port where the local service is listening for connections to the path.
  - `listener_port` `(int: 0)` - The port where the proxy will listen for connections. This port must be  available for 
  the listener to be set up. If the port is not free then Envoy will not expose a listener for the path, 
  but the proxy registration will not fail.
  - `protocol` `(string: "http")` - Sets the protocol of the listener. One of `http` or `http2`. For gRPC use `http2`.
