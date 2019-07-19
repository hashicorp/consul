---
layout: "docs"
page_title: "Consul Enterprise"
sidebar_current: "docs-enterprise"
description: |-
  Consul Enterprise features a number of capabilities beyond the open source offering that may be beneficial in certain workflows.
---

# Consul Enterprise

Consul Enterprise simplifies operations by automating workflows. It adds support
for microservices deployments across complex network topologies. It also
increases both scalability and resilience. If you have already purchased Consul Enterprise, please see the [licensing section](#licensing)
below.

Features include:

- [Automated Backups](/docs/enterprise/backups/index.html)
- [Automated Upgrades](/docs/enterprise/upgrades/index.html)
- [Enhanced Read Scalability](/docs/enterprise/read-scale/index.html)
- [Redundancy Zones](/docs/enterprise/redundancy/index.html)
- [Advanced Federation for Complex Network
  Topologies](/docs/enterprise/federation/index.html)
- [Network Segments](/docs/enterprise/network-segments/index.html)
- [Sentinel](/docs/enterprise/sentinel/index.html)

These features are part of [Consul
Enterprise](https://www.hashicorp.com/consul.html).

## Licensing

Licensing capabilities were added to Consul Enterprise v1.1.0. The license is set
once for a datacenter and will automatically propagate to all nodes within the
datacenter over a period of time scaled between 1 and 20 minutes depending on the
number of nodes in the datacenter. There are two methods for licensing Consul
enterprise.

### Included in the Enterprise Package

If you are downloading Consul from Amazon S3, then the license is included
in the binary and you do not need to take further action. This is the 
most common use case.

In the S3 bucket you will find three Enterprise zip packages. The packages with `+pro` and
`+prem` in the name, are the binaries that include the license. The package
with `+ent` in the name does not include the license.

### Applied after Bootstrapping

If you are downloading the enterprise binary from the [releases.hashicorp.com](https://releases.hashicorp.com/consul/) or the `+ent` package from Amazon S3, you will need to apply
the license to the cluster, after completing the bootstrapping process. 
You can set the license on any agent within the cluster and it will be 
forwarded to the leading server via the RPC forwarding functionality. 
Below are your two options for setting the license file. 

You can set the license via the 
[API](/api/operator/license.html) or the [CLI](/docs/commands/license.html). When
you first start Consul, a 30-minute temporary license is available to allow you
time to license the datacenter. You should set the license within ten minutes of
starting the first Consul process to allow time for the license to propagate.
