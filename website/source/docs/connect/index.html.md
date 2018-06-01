---
layout: "docs"
page_title: "Connect (Service Segmentation)"
sidebar_current: "docs-connect-index"
description: |-
  Consul Connect provides service-to-service connection authorization and encryption using mutual TLS.
---

# Connect

Consul Connect provides service-to-service connection authorization
and encryption using mutual TLS. Applications can use
[sidecar proxies](/docs/connect/proxies.html)
to automatically establish TLS connections for inbound and outbound connections
without being aware of Connect at all. Applications may also
[natively integrate with Connect](/docs/connect/native.html)
for optimal performance and security.

Connect enables deployment best-practices with service-to-service encryption
everywhere and identity-based authorization. Rather than authorizing host-based
access with IP address access rules, Connect uses the registered service
identity to enforce access control with [intentions](/docs/connect/intentions.html).
This makes it much easier to reason about access control and also enables
services to freely move, such as in a scheduled environment with software
such as Kubernetes or Nomad. Additionally, intention enforcement can be done
regardless of the underlying network, so Connect works with physical networks,
cloud networks, software-defined networks, cross-cloud, and more.

## How it Works

The core of Connect is based on [mutual TLS](https://en.wikipedia.org/wiki/Mutual_authentication).

Connect provides each service with an identity encoded as a TLS certificate.
This certificate is used to establish and accept connections to and from other
services. The identity is encoded in the TLS certificate in compliance with
the [SPIFFE X.509 Identity Document](https://github.com/spiffe/spiffe/blob/master/standards/X509-SVID.md).
This enables Connect services to establish and accept connections with
other SPIFFE-compliant systems.

The client service verifies the destination service certificate
against the [public CA bundle](/api/connect/ca.html#list-ca-root-certificates).
This is very similar to a typical HTTPS web browser connection. In addition
to this, the client provides its own client certificate to show its
identity to the destination service. If the connection handshake succeeds,
the connection is encrypted and authorized.

The destination service verifies the client certificate
against the [public CA bundle](/api/connect/ca.html#list-ca-root-certificates).
After verifying the certificate, it must also call the
[authorization API](/api/agent/connect.html#authorize) to authorize
the connection against the configured set of Consul intentions.
If the authorization API responds successfully, the connection is established.
Otherwise, the connection is rejected.

To generate and distribute certificates, Consul has a built-in CA that
requires no other dependencies, and
also ships with built-in support for [Vault](#). The PKI system is pluggable
and can be [extended](#) to support any system.

All APIs required for Connect typically respond in microseconds and impose
minimal overhead to existing services. This is because the Connect-related
APIs are all made to the local Consul agent over a loopback interface, and all
[agent Connect endpoints](/api/agent/connect.html) implement
local caching, background updating, and support blocking queries. As a result,
most API calls operate on purely local in-memory data and can respond
in microseconds.
