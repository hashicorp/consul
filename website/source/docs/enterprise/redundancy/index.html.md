---
layout: "docs"
page_title: "Consul Enterprise Redundancy Zones"
sidebar_current: "docs-enterprise-redundancy"
description: |-
  Consul Enterprise redundancy zones enable hot standby servers on a per availability zone basis.
---

# Consul Enterprise Redundancy Zones

[Consul Enterprise](https://www.hashicorp.com/consul.html) [redundancy
zones](/docs/guides/autopilot.html#redundancy-zones) make
it possible to have more servers than availability zones. For example, in an
environment with three availability zones it's now possible to run one voter and
one non-voter in each availability zone, for a total of six servers. If an
availability zone is completely lost, only one voter will be lost, so the
cluster remains available. If a voter is lost in an availability zone, Autopilot
will promote the non-voter to voter automatically, putting the hot standby
server into service quickly.
