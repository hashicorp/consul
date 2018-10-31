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

Intention replication happens automatically but requires the [`PrimaryDatacenter`](/docs/agent/options.html#primary_datacenter)
configuration to be set to specify a datacenter that is the authorative DC
for intentions.

This primary datacenter also acts as the root Certificate Authority for Connect.
The built-in CA generates the cluster root key and trust domain UUID. Non-authoritative
datacenters will then replicate the public CA certificate and trust-domain from
the authority, and generate a CSR for an intermediate key they can use to sign
certificates locally to ensure no dependency on WAN connectivity for normal
operation.

Writes and updates made to non-authoritative datacenters will not be replicated back
to the primary. All intention updates should be made to this primary datacenter.
Intentions are replicated to other DCs using blocking watches.

## Configuration

Configuration for multi-dc intentions mirrors that of normal configuration,
but a `PrimaryDatacenter` must be specified.

```
...
  "primary_datacenter": "dc1",
...
```

Assuming a multi-dc setup with two datacenters (`dc1`, `dc2`) the built-in proxy
can then specify upstream services via the `dc` flag:

```
$ consul connect proxy \
         -service client
         -upstream nginx:80
         -datacenter dc2
```

Alternatively, upstreams can be [prepared queries](/api/query.html) that resolve
services across datacenters.
