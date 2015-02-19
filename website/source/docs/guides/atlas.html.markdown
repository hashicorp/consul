---
layout: "docs"
page_title: "Atlas Integration"
sidebar_current: "docs-guides-atlas"
description: |-
  This guide covers how to integrate Atlas with Consul to provide features like an infrastructure dashboard and automatic cluster joining.
---

# Atlas Integration

[Atlas](https://atlas.hashicorp.com) is service provided by HashiCorp to deploy applications and manage infrastructure.
Starting with Consul 0.5, it is possible to integrate Consul with Atlas. This is done by registering a node as part
of an Atlas infrastructure (specified with the `-atlas` flag). Consul maintains a long running connection to the
[SCADA](http://scada.hashicorp.com) service which allows Atlas to retrieve data and control nodes.

Data acquisition allows Atlas to display the state of the Consul cluster in its dashboard as well as enabling
alerts to be setup using health checks. Remote control enables Atlas to provide features like the auto joinining
nodes.

## Enabling Atlas Integration

To enable Atlas integration, you must specify the name of the Atlas infrastructure and the Atlas authentication
token. The Atlas infrastructure name can be set either with the `-atlas` CLI flag, or with the `atlas_infrastructure`
[configuration option](/docs/agent/options.html). The Atlas token is set with the `-atlas-token` CLI flag, `atlas_token`
configuration option, or `ATLAS_TOKEN` environment variable.

To verify the integration, either run the agent with `debug` level logging or use `consul monitor -log-level=debug`
and look for a line like:

    [DEBUG] scada-client: assigned session '406ca55d-1801-f964-2942-45f5f9df3995'

This shows that the Consul agent was successfully able to register with the SCADA service.

## Using Auto-Join

Once integrated with Atlas, the auto join feature can be used to have nodes automatically join other
peers in their datacenter. Server nodes will automatically join peer LAN nodes and other WAN nodes.
Client nodes will only join other LAN nodes in their datacenter.

Auto join is enabled with the `-atlas-join` CLI flag or the `atlas_join` configuration option.

## Securing Atlas

The connection to Atlas does not have elevated privileges. API requests made by Atlas
are served in the same way any other HTTP request is made. If ACLs are enabled, it is possible to
force an Atlas ACL token to be used instead of the agent's default token.

When ACLs are enabled, the `atlas_acl_token` configuration option can be specified. This changes
the ACL token resolution order to be:

1. Request specific token provided by `?token=`. These tokens are set in the Atlas UI.
2. The `atlas_acl_token` if configured.
3. The `acl_token` if configured.
4. The `anonymous` token.

Because the `acl_token` typically has elevated permissions compared to the `anonymous` token,
the `atlas_acl_token` can be set to `anonymous` to drop privileges that would otherwise be
inherited from the agent.

