---
layout: api
page_title: Operator - HTTP API
sidebar_current: api-operator
description: |-
  The /operator endpoints provide cluster-level tools for Consul operators,
  such as interacting with the Raft subsystem.
---

# Operator HTTP Endpoint

The `/operator` endpoints provide cluster-level tools for Consul operators,
such as interacting with the Raft subsystem. For a CLI to perform these
operations manually, please see the documentation for the
[`consul operator`](/docs/commands/operator.html) command.

If ACLs are enabled then a token with operator privileges may be required in
order to use this interface. See the [ACL Rules documentation](/docs/acl/acl-rules.html#operator-rules)
for more information.

See the [Outage Recovery](https://learn.hashicorp.com/consul/day-2-operations/outage) guide for some examples of
how these capabilities are used.

Please choose a sub-section in the navigation for more information.
