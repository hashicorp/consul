---
layout: "docs"
page_title: "Consul Enterprise Automated Backups"
sidebar_current: "docs-enterprise-backups"
description: |-
  Consul Enterprise provides a highly available service that manages taking snapshots, rotation and sending backup files offsite to Amazon S3.
---

# Consul Enterprise Automated Backups

Consul's core snapshot functionality allows operators to save and restore the state of
the Consul servers for disaster recovery. Snapshots are atomic and
point-in-time, and include key/value entries, service catalog, prepared
queries, sessions, and ACLs.

[Consul Enterprise](https://www.hashicorp.com/consul.html) provides a [highly
available service](/docs/commands/snapshot/agent.html) that
integrates with the snapshot API to automatically manage taking snapshots,
perform rotation and send backup files offsite to Amazon S3.
