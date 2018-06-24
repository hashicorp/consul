---
layout: "docs"
page_title: "Connect in Production"
sidebar_current: "docs-guides-connect-production"
description: |-
  This guide describes best practices for running Consul Connect in production.
---

## Running Connect in Production

Consul Connect can secure all inter-service communication via mutual TLS. It's
designed to work with [minimal configuration out of the
box](/intro/getting-started/connect.html), but completing the [security
checklist](/docs/connect/security.html) and understanding the [Consul security
model](/docs/internals/security.html) are prerequisites for production
deployments.

This guide aims to walk through the steps required to ensure the security
guarantees hold.

We assume a cluster is already running with an appropriate number of servers and
clients and that other reference material like the
[deployment](/docs/guides/deployment.html) and
[performance](/docs/guides/performance.html) guides have been followed.

In practical deployments it may be necessary to incrementally adopt Connect
service-by-service. In this case some or all of the advice below may not apply
during the transition but should give a good understanding on which security
properties have been sacrificed in the interim. The final deployment goal should
be to end up compliant with all the advice below.

The steps we need to get to a secure Connect cluster are:

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
will be able to identify as that service and access all of the resources that that
service is allowed to access.

A secure ACL setup must meet these criteria:

 1. **[ACL default
    policy](/docs/agent/options.html#acl_default_policy)
    must be `deny`.** It is technically sufficient to keep the default policy of
    `allow` but add an explicit ACL denying anonymous `service:write`. Note
    however that in this case the Connect intention graph will also default to
    `allow` and explicit `deny` intentions will be needed to restrict service
    access. Also note that explicit rules to limit who can manage intentions are
    necessary in this case. It is assumed for the remainder of this guide that
    ACL policy defaults to `deny`.
 2. **Each service must have a distinct ACL token** that is restricted to
    `service:write` only for the named service. Current Consul ACLs only support
    prefix matching but in a near-future release we will allow exact name
    matching. It is possible for all instances of the service to share the same
    token although best practices is for each instance to get a unique token as
    described below.

### Fine Grained Enforcement

Connect intentions manage access based only on service identity so it is
sufficient for ACL tokens to only be unique per _service_ and shared between
instances.

It is much better though if ACL tokens are unique per service _instance_ because
it limits the blast radius of a compromise.

A future release of Connect will support revoking specific certificates that
have been issued. For example if a single node in a datacenter has been
compromised, it will be possible to find all certificates issued to the agent on
that node and revoke them. This will block all access to the intruder without
taking instances of the service(s) on other nodes offline too.

While this will work with service-unique tokens, there is nothing stopping an
attacker from obtaining certificates while spoofing the agent ID or other
identifier – these certificates will not appear to have been issued to the
compromised agent and so will not be revoked.

If every service instance has a unique token however, it will be possible to
revoke all certificates that were requested under that token. Assuming the
attacker can only access the tokens present on the compromised host, this
guarantees that any certificate they might have access to or requested directly
will be revoked.

In practice, managing per-instance tokens requires automated ACL provisioning,
for example using [HashiCorp's
Vault](https://www.vaultproject.io/docs/secrets/consul/index.html).

## Configure Agent Transport Encryption

Consul's gossip (UDP) and RPC (TCP) communications need to be encrypted
otherwise attackers may be able to see ACL tokens while in flight
between the server and client agents (RPC) or between client agent and 
application (HTTP). Certificate private keys never leave the host they 
are used on but are delivered to the application or proxy over local 
HTTP so local agent traffic should be encrypted where potentially 
untrusted parties might be able to observe localhost agent API traffic.

Follow the [encryption documentation](/docs/agent/encryption.html) to ensure
both gossip encryption and RPC/HTTP TLS are configured securely.

For now client and server TLS certificates are still managed by manual
configuration. In the future we plan to automate more of that with the same
mechanisms Connect offers to user applications.

## Bootstrap Certificate Authority

Consul Connect comes with a built in Certificate Authority (CA) that will
bootstrap by default when you first enable Connect on your servers.

To use the built-in CA, enable it in the server's configuration.

```text
connect {
  enabled = true
}
```

This config change requires a restart which you can perform one server at a time
to maintain availability in an existing cluster.

As soon as a server that has Connect enabled becomes the leader, it will
bootstrap a new CA and generate it's own private key which is written to the
Raft state.

Alternatively, an external private key can be provided via the [CA
configuration](/docs/connect/ca.html#specifying-a-private-key-and-root-certificate).

### External CAs

Connect has been designed with a pluggable CA component so external CAs can be
integrated. We will expand the external CA systems that are supported in the
future and will allow seamless online migration to a different CA or
bootstrapping with an external CA.

For production workloads we recommend using [Vault or another external
CA](/docs/connect/ca.html#external-ca-certificate-authority-providers) once
available such that the root key is not stored within Consul state at all.

## Setup Host Firewall

In order to enable inbound connections to connect proxies, you may need to
configure host or network firewalls to allow incoming connections to proxy
ports.

In addition to Consul agent's [communication
ports](/docs/agent/options.html#ports) any
[managed proxies](/docs/connect/proxies.html#managed-proxies) will need to have
ports open to accept incoming connections.

Consul will by default assign them ports from [a configurable
range](/docs/agent/options.html#ports) the default
range is 20000 - 20255. If this feature is used, the agent assumes all ports in
that range are both free to use (no other processes listening on them) and are
exposed in the firewall to accept connections from other service hosts.

Alternatively, managed proxies can have their public ports specified as part of
the [proxy
configuration](/docs/connect/configuration.html#local_bind_port) in the
service definition. It is possible to use this exclusively and prevent
automated port selection by [configuring `proxy_min_port` and
`proxy_max_port`](/docs/agent/options.html#ports) to both be `0`, forcing any
managed proxies to have an explicit port configured.

It then becomes the same problem as opening ports necessary for any other
application and might be managed by configuration management or a scheduler.

## Configure Service Instances

With [necessary ACL tokens](#configure-acls) in place, all service registrations
need to have an appropriate ACL token present.

For on-disk configuration the `token` parameter of the service definition must
be set.

For registration via the API [the token is passed in the request
header](/api/index.html#acls) or by using the [Go
client configuration](https://godoc.org/github.com/hashicorp/consul/api#Config).
Note that by default API registration will not allow managed proxies to be
configured since it potentially opens a remote execution vulnerability if the
agent API endpoints are publicly accessible. This can be [configured
per-agent](/docs/agent/options.html#connect_proxy).

For examples of service definitions with managed or unmanaged proxies see
[proxies documentation](/docs/connect/proxies.html#managed-proxies).

To avoid the overhead of a proxy, applications may [natively
integrate](/docs/connect/native.html) with connect.

### Protect Application Listener

If using any kind of proxy for connect, the application must ensure no untrusted
connections can be made to it's unprotected listening port. This is typically
done by binding to `localhost` and only allowing loopback traffic, but may also
be achieved using firewall rules or network namespacing.
