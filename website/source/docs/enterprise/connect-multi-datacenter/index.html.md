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
and CA provider state.

Upstream proxies may specify an alternative datacenter or a prepared
query that can address services in multiple DCs (such as
the [geo failover](/docs/guides/geo-failover.html) pattern) as an upstream.

[Intentions](/docs/connect/intentions.html) verify connections between services by source and destination
name while authorizing across DCs.

# Replication

Intention replication happens automatically but requires the [`primary_datacenter`](/docs/agent/options.html#primary_datacenter)
configuration to be set to specify a datacenter that is the authorative DC
for intentions. Intentions are replicated to other DCs using blocking watches.

This primary datacenter also acts as the root Certificate Authority for Connect. 
The primary datacenter then generates a trust-domain UUID and obtains a root 
certificate from the CA provider. Secondary datacenters will then replicate the 
root CA public key and trust-domain ID from the primary and generate their own key 
and CSR for an intermediate CA certificate. This CSR is signed by the primary and 
used to issue new Connect certificates in the secondary DC without WAN RPCs. No CA 
keys are replicated between datacenters.
