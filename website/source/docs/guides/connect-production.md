---
layout: "docs"
page_title: "Connect in Production"
sidebar_current: "docs-guides-connect-production"
description: |-
  This guide describes best practices for running Consul Connect in production.
---

# Running Connect in Production

Consul Connect can secure all inter-service communication with mutual TLS. It's
designed to work with [minimal configuration out of the
box](https://learn.hashicorp.com/consul/getting-started/connect), however, completing the [security
checklist](/docs/connect/security.html) and understanding the [Consul security
model](/docs/internals/security.html) are prerequisites for production
deployments.

After completing this guide, you will be able to configure Connect to 
secure services. First, you will secure your Consul cluster with ACLs and
TLS encryption. Next, you will configure Connect on the servers and host. 
Finally, you will configure your services to use Connect.

~> Note: To complete this guide you should already have a Consul cluster
 with an appropriate number of servers and
clients deployed according to the other reference material including the
[deployment](/docs/guides/deployment.html) and
[performance](/docs/install/performance.html) guides.

The steps we need to get to a secure Connect cluster are:

 1. [Configure ACLs](#configure-acls)
 1. [Configure Agent Transport Encryption](#configure-agent-transport-encryption)
 1. [Bootstrap Connect's Certificate Authority](#bootstrap-certificate-authority)
 1. [Setup Host Firewall](#setup-host-firewall)
 1. [Configure Service Instances](#configure-service-instances)

For existing Consul deployments, it may be necessary to incrementally adopt Connect
service-by-service. In this case, step one and two should already be complete. 
However, we recommend reviewing all steps since the final deployment goal is to be compliant with all the security recommendations in this guide.

## Configure ACLs

Consul Connect's security is based on service identity. In practice, the identity
of the service is only enforcible with sufficiently restrictive ACLs.

This section will not replace reading the full [ACL
guide](/docs/guides/acl.html) but will highlight the specific requirements
Connect relies on to ensure it's security properties.

A service's identity, in the form of an x.509 certificate, will only be issued
to an API client that has `service:write` permission for that service. In other
words, any client that has permission to _register_ an instance of a service
will be able to identify as that service and access all of the resources that that
service is allowed to access.

A secure ACL setup must meet the following criteria.

 1. **[ACL default
    policy](/docs/agent/options.html#acl_default_policy)
    must be `deny`.** If for any reason you cannot use the default policy of
    `deny`, you must add an explicit ACL denying anonymous `service:write`. Note, in this case the Connect intention graph will also default to
    `allow` and explicit `deny` intentions will be needed to restrict service
    access. Also note that explicit rules to limit who can manage intentions are
    necessary in this case. It is assumed for the remainder of this guide that
    ACL policy defaults to `deny`.
 2. **Each service must have a unique ACL token** that is restricted to
    `service:write` only for the named service. You can review the [Securing Consul with ACLs](https://learn.hashicorp.com/consul/advanced/day-1-operations/production-acls#apply-individual-tokens-to-the-services) guide for a 
    service token example. Note, it is best practices for each instance to get a unique token as described below.

~> Individual Service Tokens: It is best practice to create a unique ACL token per service _instance_ because
it limits the blast radius of a compromise. However, since Connect intentions manage access based only on service identity, it is
possible to create only one ACL token per _service_ and share it between
instances.

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

Follow the [encryption guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/agent-encryption) to ensure
both gossip encryption and RPC/HTTP TLS are configured securely.

## Bootstrap Connect's Certificate Authority

Consul Connect comes with a built-in Certificate Authority (CA) that will
bootstrap by default when you first [enable](https://www.consul.io/docs/agent/options.html#connect_enabled) Connect on your servers.

To use the built-in CA, enable it in the server's configuration.

```text
connect {
  enabled = true
}
```

This configuration change requires a Consul server restart, which you can perform one server at a time
to maintain availability in an existing cluster.

As soon as a server that has Connect enabled becomes the leader, it will
bootstrap a new CA and generate it's own private key which is written to the
Raft state.

Alternatively, an external private key can be provided via the [CA
configuration](/docs/connect/ca.html#specifying-a-private-key-and-root-certificate).

~> External CAs: Connect has been designed with a pluggable CA component so external CAs can be
integrated. For production workloads we recommend using [Vault or another external
CA](/docs/connect/ca.html#external-ca-certificate-authority-providers) once
available such that the root key is not stored within Consul state at all.

## Setup Host Firewall

In order to enable inbound connections to connect proxies, you may need to
configure host or network firewalls to allow incoming connections to proxy
ports.

In addition to Consul agent's [communication
ports](/docs/agent/options.html#ports) any
[proxies](/docs/connect/proxies.html) will need to have
ports open to accept incoming connections.

If using [sidecar service
registration](/docs/connect/proxies/sidecar-service.html) Consul will by default
assign ports from [a configurable
range](/docs/agent/options.html#sidecar_min_port) the default range is 21000 -
21255. If this feature is used, the agent assumes all ports in that range are
both free to use (no other processes listening on them) and are exposed in the
firewall to accept connections from other service hosts.

It is possible to prevent automated port selection by [configuring
`sidecar_min_port` and
`sidecar_max_port`](/docs/agent/options.html#sidecar_min_port) to both be `0`,
forcing any sidecar service registrations to need an explicit port configured.

It then becomes the same problem as opening ports necessary for any other
application and might be managed by configuration management or a scheduler.

## Configure Service Instances

With [necessary ACL tokens](#configure-acls) in place, all service registrations
need to have an appropriate ACL token present.

For on-disk configuration the `token` parameter of the service definition must
be set. 

```json
{ 
  "service": { 
    "name": "cassandra_db", 
    "port": 9002, 
    "token: "<your_token_here>"
    } 
 }
```

For registration via the API the token is passed in the [request
header](/api/index.html#authentication), `X-Consul-Token`, or by using the [Go
client configuration](https://godoc.org/github.com/hashicorp/consul/api#Config).

To avoid the overhead of a proxy, applications may [natively
integrate](/docs/connect/native.html) with connect.

~> Protect Application Listener: If using any kind of proxy for connect, the application must ensure no untrusted
connections can be made to it's unprotected listening port. This is typically
done by binding to `localhost` and only allowing loopback traffic, but may also
be achieved using firewall rules or network namespacing.

For examples of proxy service definitions see the [proxy
documentation](/docs/connect/proxies.html).

## Summary

After securing your Consul cluster with ACLs and TLS encryption, you 
can use Connect to secure service-to-service communication. If you
encounter any issues while setting up Consul Connect, there are 
many [community](https://www.consul.io/community.html) resources where you can find help.


