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
also ships with built-in support for [Vault](/docs/connect/ca/vault.html). The PKI system is designed to be pluggable
and can be extended to support any system by adding additional CA providers.

All APIs required for Connect typically respond in microseconds and impose
minimal overhead to existing services. This is because the Connect-related
APIs are all made to the local Consul agent over a loopback interface, and all
[agent Connect endpoints](/api/agent/connect.html) implement
local caching, background updating, and support blocking queries. As a result,
most API calls operate on purely local in-memory data and can respond
in microseconds.

## Getting Started With Connect

There are several ways to try Connect in different environments.

 * The [Connect introduction](https://learn.hashicorp.com/consul/getting-started/connect) in the
   Getting Started guide provides a simple walk through of getting two services
   to communicate via Connect using only Consul directly on your local machine.

 * The [Envoy guide](/docs/guides/connect-envoy.html) walks through getting
   started with Envoy as a proxy, and uses Docker to run components locally
   without installing anything else.

 * The [Kubernetes documentation](/docs/platform/k8s/run.html) shows how to get
   from an empty Kubernetes cluster to having Consul installed and Envoy
   configured to proxy application traffic automatically using the official helm
   chart.

## Agent Caching and Performance

To enable microsecond-speed responses on
[agent Connect API endpoints](/api/agent/connect.html), the Consul agent
locally caches most Connect-related data and sets up background
[blocking queries](/api/index.html#blocking-queries) against the server
to update the cache in the background. This allows most API calls such
as retrieving certificates or authorizing connections to use in-memory
data and respond very quickly.

All data cached locally by the agent is populated on demand. Therefore,
if Connect is not used at all, the cache does not store any data. On first
request, the data is loaded from the server and cached. The set of data cached
is: public CA root certificates, leaf certificates, and intentions. For
leaf certificates and intentions, only data related to the service requested
is cached, not the full set of data.

Further, the cache is partitioned by ACL token and datacenters. This is done
to minimize the complexity of the cache and prevent bugs where an ACL token
may see data it shouldn't from the cache. This results in higher memory usage
for cached data since it is duplicated per ACL token, but with the benefit
of simplicity and security.

With Connect enabled, you'll likely see increased memory usage by the
local Consul agent. The total memory is dependent on the number of intentions
related to the services registered with the agent accepting Connect-based
connections. The other data (leaf certificates and public CA certificates)
is a relatively fixed size per service. In most cases, the overhead per
service should be relatively small: single digit kilobytes at most.

The cache does not evict entries due to memory pressure. If memory capacity
is reached, the process will attempt to swap. If swap is disabled, the Consul
agent may begin failing and eventually crash. Cache entries do have TTLs
associated with them and will evict their entries if they're not used. Given
a long period of inactivity (3 days by default), the cache will empty itself.

## Multi-Datacenter

Using Connect for service-to-service communications across multiple datacenters 
requires Consul Enterprise. 

With Open Source Consul, Connect may be enabled on multiple Consul datacenters, 
but only services within the same datacenter can establish Connect-based, 
Authenticated and Authorized connections. In this version, Certificate Authority
configurations and intentions are both local to their respective datacenters; 
they are not replicated across datacenters.

Full multi-datacenter support for Connect is available in
[Consul Enterprise](/docs/enterprise/connect-multi-datacenter/index.html).

