---
layout: "docs"
page_title: "Connect - Architecture"
sidebar_current: "docs-connect-internals"
description: |-
  This page details the internals of Consul Connect: mutual TLS, agent caching and performance, intention and certificate authority replication.
---

# How Connect Works

This page details the inner workings of some of Connect's core features.
Understanding how these features work isn't a prerequisite for using Connect,
but will help you build a mental model of what's going on under the hood, which
may help you reason about Connect's behavior in more complex deployment
scenarios.

## Mutual Transport Layer Security (mTLS)

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
minimal overhead to existing services. This is because the Connect-related APIs
are all made to the local Consul agent over a loopback interface, and all [agent
Connect endpoints](/api/agent/connect.html) implement local caching, background
updating, and support blocking queries. Most API calls operate on purely local
in-memory data.

## Agent Caching and Performance

To enable fast responses on [agent Connect API
endpoints](/api/agent/connect.html), the Consul agent locally caches most
Connect-related data and sets up background [blocking
queries](/api/features/blocking.html) against the server to update the cache in
the background. This allows most API calls such as retrieving certificates or
authorizing connections to use in-memory data and respond very quickly.

All data cached locally by the agent is populated on demand. Therefore, if
Connect is not used at all, the cache does not store any data. On first request,
the data is loaded from the server and cached. The set of data cached is: public
CA root certificates, leaf certificates, intentions, and service discovery
results for upstreams. For leaf certificates and intentions, only data related
to the service requested is cached, not the full set of data.

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

## Connections Across Datacenters

Sidecar proxy's [upstream configuration](/docs/connect/registration/service-registration.html#upstream-configuration-reference)
may specify an alternative datacenter or a prepared query that can address services
in multiple datacenters (such as the [geo failover](https://learn.hashicorp.com/consul/developer-discovery/geo-failover) pattern).

[Intentions](/docs/connect/intentions.html) verify connections between services by
source and destination name seamlessly across datacenters.

Connections can be made via gateways to enable when communciating across
network topologies allowing connections between services in each datacenter
without externally routable IPs at the service level.

## Intention Replication

Intention replication happens automatically but requires the
[`primary_datacenter`](/docs/agent/options.html#primary_datacenter)
configuration to be set to specify a datacenter that is authoritative
for intentions. In production setups with ACLs enabled, the
[replication token](/docs/agent/options.html#acl_tokens_replication) must also
be set in the secondary datacenter server's configuration.

## Certificate Authority Federation

The primary datacenter also acts as the root Certificate Authority (CA) for Connect.
The primary datacenter generates a trust-domain UUID and obtains a root certificate
from the configured CA provider which defaults to the built-in one.

Secondary datacenters fetch the root CA public key and trust-domain ID from the
primary and generate their own key and Certificate Signing Request (CSR) for an
intermediate CA certificate. This CSR is signed by the root in the primary
datacenter and the certificate is returned. The secondary datacenter can now use
this intermediate to sign new Connect certificates in the secondary datacenter
without WAN communication. CA keys are never replicated between datacenters.

The secondary maintains watches on the root CA certificate in the primary. If the
CA root changes for any reason such as rotation or migration to a new CA, the
secondary automatically generates new keys and has them signed by the primary
datacenter's new root before initiating an automatic rotation of all issued
certificates in use throughout the secondary datacenter. This makes CA root key
rotation fully automatic and with zero downtime across multiple datacenters.
