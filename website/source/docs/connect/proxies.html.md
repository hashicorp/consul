---
layout: "docs"
page_title: "Connect - Proxies"
sidebar_current: "docs-connect-proxies"
description: |-
  A Connect-aware proxy enables unmodified applications to use Connect. This section details how to use either Envoy or Consul's built-in L4 proxy, and describes how you can plug in a proxy of your choice.
---

# Connect Proxies

A Connect-aware proxy enables unmodified applications to use Connect. A
per-service proxy sidecar transparently handles inbound and outbound service
connections, automatically wrapping and verifying TLS connections. Consul
includes its own built-in L4 proxy and has first class support for Envoy. You
can choose other proxies to plug in as well. This section describes how to
configure Envoy or the built-in proxy using Connect, and how to integrate the
proxy of your choice.

To ensure that services only allow external connections established via
the Connect protocol, you should configure all services to only accept connections on a loopback address.

~> **Deprecation Note:** Managed Proxies are a deprecated method for deploying
sidecar proxies, and have been removed in Consul 1.6. See [managed proxy
deprecation](/docs/connect/proxies/managed-deprecated.html) for more
information. If you are using managed proxies we strongly recommend that you
switch service definitions for registering proxies.

## Dynamic Upstreams Require Native Integration

If an application requires dynamic dependencies that are only available
at runtime, it must [natively integrate](/docs/connect/native.html)
with Connect. After natively integrating, the HTTP API or
[DNS interface](/docs/agent/dns.html#connect-capable-service-lookups)
can be used.

!> Connect proxies do not currently support dynamic upstreams.
