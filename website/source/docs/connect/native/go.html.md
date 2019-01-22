---
layout: "docs"
page_title: "Connect - Native Application Integration - Go"
sidebar_current: "docs-connect-native-go"
description: |-
  We provide a library that makes it drop-in simple to integrate Connect with most Go applications. For most Go applications, Connect can be natively integrated in just a single line of code excluding imports and struct initialization.
---

# Connect-Native Integration with Go

We provide a library that makes it drop-in simple to integrate Connect
with most [Go](https://golang.org/) applications. This page shows examples
of integrating this library for accepting or establishing Connect-based
connections. For most Go applications, Connect can be natively integrated
in just a single line of code excluding imports and struct initialization.

In addition to this, please read and understand the
[overview of Connect-Native integrations](/docs/connect/native.html).
In particular, after integrating applications with Connect, they must declare
that they accept Connect-based connections via their service definitions.

## Accepting Connections

Any server that supports TLS (HTTP, gRPC, net/rpc, etc.) can begin
accepting Connect-based connections in just a few lines of code. For most
existing applications, converting the server to accept Connect-based
connections will require only a one-line change excluding imports and
structure initialization.

The
Go library exposes a `*tls.Config` that _automatically_ communicates with
Consul to load certificates and authorize inbound connections during the
TLS handshake. This also automatically starts goroutines to update any
changing certs.

Example, followed by more details:

```go
import(
  "net/http"

  "github.com/hashicorp/consul/api"
  "github.com/hashicorp/consul/connect"
)

func main() {
  // Create a Consul API client
  client, _ := api.NewClient(api.DefaultConfig())

  // Create an instance representing this service. "my-service" is the
  // name of _this_ service. The service should be cleaned up via Close.
  svc, _ := connect.NewService("my-service", client)
  defer svc.Close()

  // Creating an HTTP server that serves via Connect
  server := &http.Server{
    Addr:      ":8080",
    TLSConfig: svc.ServerTLSConfig(),
    // ... other standard fields
  }

  // Serve!
  server.ListenAndServeTLS("", "")
}
```

The first step is to create a Consul API client. This is almost always the
default configuration with an ACL token set, since you want to communicate
to the local agent. The default configuration will also read the ACL token
from environment variables if set. The Go library will use this client to request certificates,
authorize connections, and more.

Next, `connect.NewService` is called to create a service structure representing
the _currently running service_. This structure maintains all the state
for accepting and establishing connections. An application should generally
create one service and reuse that one service for all servers and clients.

Finally, a standard `*http.Server` is created. The magic line is the `TLSConfig`
value. This is set to a TLS configuration returned by the service structure.
This TLS configuration is configured to automatically load certificates
in the background, cache them, and authorize inbound connections. The service
structure automatically handles maintaining blocking queries to update certificates
in the background if they change.

Since the service returns a standard `*tls.Config`, _any_ server that supports
TLS can be configured. This includes gRPC, net/rpc, basic TCP, and more.
Another example is shown below with just a plain TLS listener:

```go
import(
  "crypto/tls"

  "github.com/hashicorp/consul/api"
  "github.com/hashicorp/consul/connect"
)

func main() {
  // Create a Consul API client
  client, _ := api.NewClient(api.DefaultConfig())

  // Create an instance representing this service. "my-service" is the
  // name of _this_ service. The service should be cleaned up via Close.
  svc, _ := connect.NewService("my-service", client)
  defer svc.Close()

  // Creating an HTTP server that serves via Connect
  listener, _ := tls.Listen("tcp", ":8080", svc.ServerTLSConfig())
  defer listener.Close()

  // Accept
  go acceptLoop(listener)
}
```

## HTTP Clients

For Go applications that need to Connect to HTTP-based upstream dependencies,
the Go library can construct an `*http.Client` that automatically establishes
Connect-based connections as long as Consul-based service discovery is used.

Example, followed by more details:

```go
import(
  "github.com/hashicorp/consul/api"
  "github.com/hashicorp/consul/connect"
)

func main() {
  // Create a Consul API client
  client, _ := api.NewClient(api.DefaultConfig())

  // Create an instance representing this service. "my-service" is the
  // name of _this_ service. The service should be cleaned up via Close.
  svc, _ := connect.NewService("my-service", client)
  defer svc.Close()

  // Get an HTTP client
  httpClient := svc.HTTPClient()

  // Perform a request, then use the standard response
  resp, _ := httpClient.Get("https://userinfo.service.consul/user/mitchellh")
}
```

The first step is to create a Consul API client and service. These are the
same steps as accepting connections and are explained in detail in the
section above. If your application is both a client and server, both the
API client and service structure can be shared and reused.

Next, we call `svc.HTTPClient()` to return a specially configured
`*http.Client`. This client will automatically established Connect-based
connections using Consul service discovery.

Finally, we perform an HTTP `GET` request to a hypothetical userinfo service.
The HTTP client configuration automatically sends the correct client
certificate, verifies the server certificate, and manages background
goroutines for updating our certificates as necessary.

If the application already uses a manually constructed `*http.Client`,
the `svc.HTTPDialTLS` function can be used to configure the
`http.Transport.DialTLS` field to achieve equivalent behavior.

### Hostname Requirements

The hostname used in the request URL is used to identify the logical service
discovery mechanism for the target. **It's not actually resolved via DNS** but
used as a logical identifier for a Consul service discovery mechanism. It has
the following specific limitations:

 * The scheme must be `https://`.
 * It must be a Consul DNS name in one of the following forms:
   * `<name>.service[.<datacenter>].consul` to discover a healthy service
     instance for a given service.
   * `<name>.query[.<datacenter>].consul` to discover an instance via
     [Prepared Query](/api/query.html).
 * The top-level domain _must_ be `.consul` even if your cluster has a custom
   `domain` configured for it's DNS interface. This might be relaxed in the
   future.
 * Tag filters for services are not currently supported (i.e.
   `tag1.web.service.consul`) however the same behaviour can be achieved using a
   prepared query.
 * External DNS names, raw IP addresses and so on will cause an error and should
   be fetched using a separate `HTTPClient`.


## Raw TLS Connection

For a raw `net.Conn` TLS connection, the `svc.Dial` function can be used.
This will establish a connection to the desired service via Connect and
return the `net.Conn`. This connection can then be used as desired.

Example:

````go
import(
  "context"

  "github.com/hashicorp/consul/api"
  "github.com/hashicorp/consul/connect"
)

func main() {
  // Create a Consul API client
  client, _ := api.NewClient(api.DefaultConfig())

  // Create an instance representing this service. "my-service" is the
  // name of _this_ service. The service should be cleaned up via Close.
  svc, _ := connect.NewService("my-service", client)
  defer svc.Close()

  // Connect to the "userinfo" Consul service.
  conn, _ := svc.Dial(context.Background(), &connect.ConsulResolver{
    Client: client,
    Name:   "userinfo",
  })
}
```

This uses a familiar `Dial`-like function to establish raw `net.Conn` values.
The second parameter to dial is an implementation of the `connect.Resolver`
interface. The example above uses the `*connect.ConsulResolver` implementation
to perform Consul-based service discovery. This also automatically determines
the correct certificate metadata we expect the remote service to serve.

## Static Addresses, Custom Resolvers

In the raw TLS connection example, you see the use of a `connect.Resolver`
implementation. This interface can be implemented to perform address
resolution. This must return the address and also the URI SAN expected
in the TLS certificate served by the remote service.

The Go library provides two built-in resolvers:

  * `*connect.StaticResolver` can be used for static addresses where no
    service discovery is required. The expected cert URI SAN must be
    manually specified.

  * `*connect.ConsulResolver` which resolves services and prepared queries
    via the Consul API. This also automatically determines the expected
    cert URI SAN.
