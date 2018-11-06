---
layout: "docs"
page_title: "Consul Enterprise Multi-Datacenter Connect"
sidebar_current: "docs-enterprise-connect-multi-datacenter"
description: |-
  Consul Enterprise supports cross datacenter connections using Consul Connect.
---

# Consul Connect Multi-Datacenter

[Consul Enterprise](https://www.hashicorp.com/consul.html) enables service-to-service
connections across multiple Consul datacenters. This includes replication of intentions
and federation of Certificate Authority trust.

Sidecar proxy's [upstream configuration](/docs/connect/proxies.html#upstream-configuration-reference) 
may specify an alternative datacenter or a prepared query that can address services 
in multiple datacenters (such as the [geo failover](/docs/guides/geo-failover.html) pattern).

[Intentions](/docs/connect/intentions.html) verify connections between services by 
source and destination name seamlessly across datacenters. Support for constraining Intentions 
by source or destination datacenter is planned for the near future.

It is assumed that workloads can communicate between datacenters via existing network 
routes and VPN tunnels, potentially using Consul's 
[`translate_wan_addrs`](/docs/agent/options.html#translate_wan_addrs) to ensure remote 
workloads discover an externally routable IP.

# Replication

Intention replication happens automatically but requires the [`primary_datacenter`](/docs/agent/options.html#primary_datacenter)
configuration to be set to specify a datacenter that is authorative
for intentions. In production setups with ACLs enabled, the [replication token](/docs/agent/options.html#acl_tokens_replication)
must also be set in secondary datacenter server's configuration.

# Certificate Authority Federation

The primary datacenter also acts as the root Certificate Authority (CA) for Connect. 
The primary datacenter generates a trust-domain UUID and obtains a root certificate 
from the configured CA provider which defaults to the built-in one. 

Secondary datacenters fetch the root CA public key and trust-domain ID from the primary and 
generate their own key and Certificate Signing Request (CSR) for an intermediate CA certificate. 
This CSR is signed by the root in the primary datacenter and the certificate is returned. 
The secondary datacenter can now use this intermediate to sign new Connect certificates 
in the secondary datacenter without WAN communication. CA keys are never replicated between 
datacenters.

The secondary maintains watches on the root CA certificate in the primary. If the CA root
changes for any reason such as rotation or migration to a new CA, the secondary automatically
generates new keys and has them signed by the primary datacenter's new root before initiating
an automatic rotation of all issued certificates in use throughout the secondary datacenter. 
This makes CA root key rotation fully automatic and with zero downtime across multiple data 
centers.
