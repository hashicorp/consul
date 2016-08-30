---
layout: "docs"
page_title: "Operator (HTTP)"
sidebar_current: "docs-agent-http-operator"
description: >
  The operator endpoint provides cluster-level tools for Consul operators.
---

# Operator HTTP Endpoint

The Operator endpoints provide cluster-level tools for Consul operators, such
as interacting with the Raft subsystem. This was added in Consul 0.7.

~> Use this interface with extreme caution, as improper use could lead to a Consul
   outage and even loss of data.

If ACLs are enabled then a token with operator privileges may required in
order to use this interface. See the [ACL](/docs/internals/acl.html#operator)
internals guide for more information.

See the [Outage Recovery](/docs/guides/outage.html) guide for some examples of how
these capabilities are used. For a CLI to perform these operations manually, please
see the documentation for the [`consul operator`](/docs/commands/operator.html)
command.

The following endpoints are supported:

* [`/v1/operator/raft/configuration`](#raft-configuration): Inspects the Raft configuration
* [`/v1/operator/raft/peer`](#raft-peer): Operates on Raft peers

Not all endpoints support blocking queries and all consistency modes,
see details in the sections below.

The operator endpoints support the use of ACL Tokens. See the
[ACL](/docs/internals/acl.html#operator) internals guide for more information.

### <a name="raft-configuration"></a> /v1/operator/raft/configuration

The Raft configuration endpoint supports the `GET` method.

#### GET Method

### <a name="raft-peer"></a> /v1/operator/raft/peer

The Raft peer endpoint supports the `DELETE` method.

#### DELETE Method

