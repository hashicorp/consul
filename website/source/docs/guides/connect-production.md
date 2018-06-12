---
layout: "docs"
page_title: "Connect in Production"
sidebar_current: "docs-guides-connect-production"
description: |-
  This guide describes best practices for running Consul Connect in production.
---

## Running Connect in Production

Consul Connect can secure all inter-service communication via mutual TLS. It's
designed to work with minimal configuration out of the box, but completing the
[security checklist](/docs/connect/security.html) and understanding the [Consul
security model](/docs/internals/security.html) are prerequisites for production
deployments.

This guide aims to walk step-by-step through a cluster setup that meets all of
those security-related goals.

We assume a cluster is already running with an appropriate number of servers and
clients. To follow along with this guide in a dev environment you can follow our
[getting started guide](/intro/getting-started/install.html). For an actual
production cluster we expect other reference material like the
[deployment](/docs/guides/deployment.html) and
[performance](/docs/guides/performance.html) guides have been followed.

The steps we need to take to get to a secure connect cluster are:

 1. [Configure ACLs](#configure-acls)
 1. [Configure Agent Transport Encryption](#configure-agent-transport-encryption)
 1. [Bootstrap Certificate Authority](#bootstrap-certificate-authority)
 1. [Setup Host Firewall](#setup-host-firewall)
 1. [Configure Service Instances](#configure-service-instances)

## Configure ACLs

Consul Connect's security is based on service identity. In practice the identity
of the service is only enforcible with sufficiently restrictive ACLs.

This section will not replace reading the full [ACL
guide](/docs/guides/acl.html) but will highlight the specific requirements
Connect relies on to ensure it's security properties.

A service's identity, in the form of an x.509 certificate, will only be issued
to an API client that has `service:write` permission for that service. In other
words, any client that has permission to _register_ an instance of a service
will be able to identify as that service and access all of resources that that
service is allowed to access.

A secure ACL setup must meet these criteria:

 1. **[ACL default
    policy](https://private-docs.consul.io/docs/agent/options.html#acl_default_policy)
    must be `deny`.** It is technically sufficient to keep default `allow` but
    add an explicit ACL denying anonymous `service:write`. Note however that in
    this case the Connect intention graph will also default to `allow` and
    explicit `deny` intentions will be needed to restrict service access. It is
    assumed for the remainder of this guide that ACL policy defaults to `deny`.
 2. **Each service must have a distinct ACL token** that is restricted to
    `service:write` only for the named service. Current Consul ACLs only support
    prefix matching but in a near-future release we will allow exact name
    matching. It is possible for all instances of the service to share the same
    token although best practices is for each instance to get a unique token as
    described below.

### Fine Grained Enforcement

Connect intentions manage access based only on service identity so it is
sufficient for ACL tokens to only be unique per service and shared between
instances.

It is much better though if ACL tokens are unique per service _instance_ though.
The reason for this is to limit the blast radius of a compromise.

A future release of Connect will support revoking specific certificates that
have been issued. For example if a single node in a datacenter has been
compromised, it will be possible to find all certificates issued to the agent on
that node and revoke them blocking access to the intruder without taking
unaffected instances of the service(s) on that node offline too.

While this will work with service-unique tokens, there is nothing stopping an
attacker from obtaining certificates while spoofing the agent ID of another
agent - these certificates will not appear to have been issued to the
compromised agent and so will not be revoked. If every service instance has a
unique token however, it will be possible to revoke all certificates that were
requested under that token which denies access to any certificate the attacker
could generate.

In practice managing per-instance tokens requires automated ACL provisioning,
for example using [HashiCorp's
Vault](https://www.vaultproject.io/docs/secrets/consul/index.html).

## Configure Agent Transport Encryption

Consul's gossip (UDP) and RPC (TCP) communications need to be encrypted
otherwise attackers may be able to see tokens and private keys while in flight
between the server and client agents or between client agent and application.

Follow the [encryption documentation](/docs/agent/encryption.html) to ensure
both gossip encryption and RPC TLS are configured securely.

## Bootstrap Certificate Authority

Consul Connect comes with a built in Certificate Authority (CA) that will
bootstrap by default when you first enable Connect on your servers.

To use the built-in CA, enable it in the server's configuration.

```text
connect {
  enabled = true
}
```

Note that server agents running in `-dev` mode have this enabled by default.

This config change requires a restart which you can perform one server at a time
to maintain availability in an existing cluster.

As soon as a server that has Connect enabled becomes the leader, it will
bootstrap a new CA and generate it's own private key which is written to the
Raft state.

Alternatively, an external private key can be provided via the [CA
configuration](#TODO).

### External CAs

Connect has been designed with a pluggable CA component so external CAs can be
integrated. We will expand the external CA systems that are supported in the
future and will allow seamless online migration to a different CA or
bootstrapping with an external CA.

For production workloads we recommend using Vault as the CA such that the root
key is not stored within Consul state at all.

## Setup Host Firewall

If using [managed proxies]() Consul will by default assign them ports from [a
configurable range]() the default range is 20000 - 20255. If this feature is
used, the agent assumes all ports in that range are both free to use (no other
processes listening on them) and are exposed in the firewall to accept
connections from other service hosts.

TODO: could show example iptables rule but it seems kinda limited and obvious

## Configure Service Instances

TODO: 
 - provide ACL token to API client/on disk
 - optionally configure manged proxy
 - notes about binding app only to localhost

