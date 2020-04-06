---
layout: 'docs'
page_title: 'Consul Enterprise Enhanced Read Scalability'
sidebar_current: 'docs-enterprise-read-scale'
description: |-
  Consul Enterprise supports increased read scalability without impacting write latency by introducing
  non-voting servers.
---

# Enhanced Read Scalability with Non-Voting Servers

[Consul Enterprise](https://www.hashicorp.com/consul.html) provides the ability to scale clustered Consul servers
to include voting and non-voting servers. Non-voting servers still receive data from the cluster replication,
however, they do not take part in quorum election operations. Expanding your Consul cluster in this way can scale
reads without impacting write latency.

For more details, review the [Consul server configuration](https://www.consul.io/docs/agent/options.html)
documentation and the [-non-voting-server](https://www.consul.io/docs/agent/options.html#_non_voting_server)
configuration flag.
