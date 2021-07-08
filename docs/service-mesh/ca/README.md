# Certificate Authority (Connect CA)

The Certificate Authority subsystem manages a CA trust chain for issuing certificates to
services and client agents (via auto-encrypt and auto-config).

The code for the Certificate Authority is in the following packages:
1. most of the core logic is in [agent/consul/leader_connect_ca.go]
2. the providers are in [agent/connect/ca]
3. the RPC interface is in [agent/consul/connect_ca_endpoint.go]


[agent/consul/leader_connect_ca.go]: https://github.com/hashicorp/consul/blob/main/agent/consul/leader_connect_ca.go
[agent/connect/ca]: https://github.com/hashicorp/consul/blob/main/agent/connect/ca/
[agent/consul/connect_ca_endpoint.go]: https://github.com/hashicorp/consul/blob/main/agent/consul/connect_ca_endpoint.go


## Architecture

### High level overview

In Consul the leader is responsible for handling of the CA management. 
When a leader election happen, and the elected leader do not have any root CA available it will start a process of creating a set of CA certificate.
Those certificates will use to authenticate/encrypt communication between services (service mesh) or between `Consul client agent` (auto-encrypt/auto-config). This process is described in the following diagram:

![CA creation](./hl-ca-overview.svg)

<sup>[source](./hl-ca-overview.mmd)</sup>

- high level explanation of what are the features that are involved in CA (mesh/connect, auto encrypt)
- add all the func that are involved in the CA operations
- relationship between the different certs


### CA and Certificate relationship

This diagram shows the relationship between the CA certificates in Consul primary and
secondary.

![CA relationship](./cert-relationship.svg)

<sup>[source](./cert-relationship.mmd)</sup>


In most cases there is an external root CA that provides an intermediate CA that Consul
uses as the Primary Root CA. The only except to this is when the Consul CA Provider is
used without specifying a `RootCert`. In this one case Consul will generate the the Root CA
from the provided primary key, and it will be used in the primary as the top of the chain
of trust.

In the primary datacenter, the Consul and AWS providers use the Primary Root CA to sign
leaf certificates. The Vault provider uses an intermediate CA to sign leaf certificates.

Leaf certificates are created for two purposes:
1. the Leaf Cert Service is used by envoy proxies in the mesh to perform mTLS with other
   services.
2. the Leaf Cert Client Agent is created by auto-encrypt and auto-config. It is used by
   client agents for HTTP API TLS, and for mTLS for RPC requests to servers.

Any secondary datacenters receive an intermediate certificate, signed by the Primary Root
CA, which is used as the CA certificate to sign leaf certificates in the secondary
datacenter.

### detailed call flow
- sequence diagram for leader election
- sequence diagram for leaf signing
- sequence diagram for CA cert rotation
