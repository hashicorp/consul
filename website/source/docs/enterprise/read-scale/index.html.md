---
layout: "docs"
page_title: "Consul Enterprise Enhanced Read Scalability"
sidebar_current: "docs-enterprise-read-scale"
description: |-
  Consul Enterprise supports increased read scalability without impacting write latency.
---

# Consul Enterprise Enhanced Read Scalability

In [Consul Enterprise](https://www.hashicorp.com/consul.html), servers can be
explicitly marked as non-voters. Non-voters will receive the replication stream
but will not take part in quorum (required by the leader before log entries can
be committed). Adding explicit non-voters will [scale
reads](/docs/guides/autopilot.html#server-read-scaling)
without impacting write latency.
