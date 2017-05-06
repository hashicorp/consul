---
layout: "docs"
page_title: "Consul Enterprise Automated Upgrades"
sidebar_current: "docs-enterprise-upgrades"
description: |-
  Consul Enterprise supports an upgrade pattern that allows operators to deploy a complete cluster of new servers and then just wait for the upgrade to complete.
---

# Consul Enterprise Automated Upgrades

[Consul Enterprise](https://www.hashicorp.com/consul.html) supports an [upgrade
pattern](/docs/guides/autopilot.html#upgrade-migrations)
that allows operators to deploy a complete cluster of new servers and then just wait
for the upgrade to complete. As the new servers join the cluster, server
introduction logic checks the version of each Consul server. If the version is
higher than the version on the current set of voters, it will avoid promoting
the new servers to voters until the number of new servers matches the number of
existing servers at the previous version. Once the numbers match, Autopilot will
begin to promote new servers and demote old ones.
