---
layout: "docs"
page_title: "Connect - Native Application Integration"
sidebar_current: "docs-connect-native"
description: |-
  Applications can natively integrate with the Connect API to support accepting and establishing connections to other Connect services without the overhead of a proxy sidecar.
---

# Connect-Native App Integration

Applications can natively integrate with the Connect API to support
accepting and establishing connections to other Connect services without
the overhead of a [proxy sidecar](/docs/connect/proxies.html). This option 
is especially useful for applications that may be experiencing performance issues 
with the proxy sidecar deployment. This page will cover the high-level overview 
of integration, registering the service, etc. For language-specific examples, 
see the sidebar navigation to the left.

Connect is just basic mutual TLS. This means that almost any application
can easily integrate with Connect. There is no custom protocol in use;
any language that supports TLS can accept and establish Connect-based
connections.

We currently provide an easy-to-use [Go integration](/docs/connect/native/go.html)
to assist with the getting the proper certificates, verifying connections,
etc. We plan to add helper libraries for other languages in the future.
However, without library support, it is still possible for any major language
to integrate with Connect.

## Overview

The primary work involved in natively integrating with Connect is
[acquiring the proper TLS certificate](/api/agent/connect.html#service-leaf-certificate),
[verifying TLS certificates](/api/agent/connect.html#certificate-authority-ca-roots),
and [authorizing inbound connections](/api/agent/connect.html#authorize).
All of this is done using the Consul HTTP APIs linked above.

An overview of the sequence is shown below. The diagram and the following
details may seem complex, but this is a _regular mutual TLS connection_ with
an API call to verify the incoming client certificate.

<div class="center">
![Native Integration Overview](connect-native-overview.png)
</div>

Details on the steps are below:

  * **Service discovery** - This is normal service discovery using Consul,
    a static IP, or any other mechanism. If you're using Consul DNS, the
    [`<service>.connect`](/docs/agent/dns.html#connect-capable-service-lookups)
    syntax to find Connect-capable endpoints for a service. After service
    discovery, choose one address from the list of **service addresses**.

  * **Mutual TLS** - As a client, connect to the discovered service address
    over normal TLS. As part of the TLS connection, provide the
    [service certificate](/api/agent/connect.html#service-leaf-certificate)
    as the client certificate. Verify the remote certificate against the
    [public CA roots](/api/agent/connect.html#certificate-authority-ca-roots).
    As a client, if the connection is established then you've established
    a Connect-based connection and there are no further steps!

  * **Authorization** - As a server accepting connections, verify the client
    certificate against the
    [public CA roots](/api/agent/connect.html#certificate-authority-ca-roots).
    After verifying the certificate, parse some basic fields from it and call
    the [authorizing API](/api/agent/connect.html#authorize) against the local
    agent. If this returns successfully, complete the TLS handshake and establish
    the connection. If authorization fails, close the connection.

-> **A note on performance:** The only API call in the connection path is
the [authorization API](/api/agent/connect.html#authorize). The other API
calls to acquire the leaf certificate and CA roots are expected to be done
out of band and reused. The authorize API call should be called against the
local Consul agent. The agent uses locally cached
data to authorize the connection and typically responds in microseconds.
Therefore, the impact to the TLS handshake is typically microseconds.

## Updating Certificates and Certificate Roots

The leaf certificate and CA roots can be updated at any time and the
natively integrated application must react to this relatively quickly
so that new connections are not disrupted. This can be done through
Consul blocking queries (HTTP long polling) or through periodic polling.

The API calls for
[acquiring a leaf TLS certificate](/api/agent/connect.html#service-leaf-certificate)
and [reading CA roots](/api/agent/connect.html#certificate-authority-ca-roots)
both support
[blocking queries](/api/index.html#blocking-queries). By using blocking
queries, an application can efficiently wait for an updated value. For example,
the leaf certificate API will block until the certificate is near expiration
or the signing certificates have changed and will issue and return a new
certificate.

In some languages, using blocking queries may not be simple. In that case,
we still recommend using the blocking query parameters but with a very short
`timeout` value set. Doing this is documented with
[blocking queries](/api/index.html#blocking-queries). The low timeout will
ensure the API responds quickly. We recommend that applications poll the
certificate endpoints frequently, such as multiple times per minute.

The overhead for the blocking queries (long or periodic polling) is minimal.
The API calls are to the local agent and the local agent uses locally
cached data multiplexed over a single TCP connection to the Consul leader.
Even if a single machine has 1,000 Connect-enabled services all blocking
on certificate updates, this translates to only one TCP connection to the
Consul server.

Some language libraries such as the
[Go library](/docs/connect/native/go.html) automatically handle updating
and locally caching the certificates.

## Service Registration

Connect-native applications must tell Consul that they support Connect
natively. This enables the service to be returned as part of service
discovery for Connect-capable services, used by other Connect-native applications
and client [proxies](/docs/connect/proxies.html).

This can be specified directly in the [service definition](/docs/agent/services.html):

```json
{
  "service": {
    "name": "redis",
    "port": 8000,
    "connect": {
      "native": true
    }
  }
}
```

Services that support Connect natively are still returned through the standard
service discovery mechanisms in addition to the Connect-only service discovery
mechanisms.
