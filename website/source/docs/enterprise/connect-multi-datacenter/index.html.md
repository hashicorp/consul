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


