---
layout: "docs"
page_title: "Autopilot"
sidebar_current: "docs-guides-autopilot"
description: |-
  This guide covers how to configure and use Autopilot features.
---

# Autopilot

Autopilot features allow for automatic,
operator-friendly management of Consul servers. It includes cleanup of dead
servers, monitoring the state of the Raft cluster, and stable server introduction.

To enable Autopilot features (with the exception of dead server cleanup),
the [`raft_protocol`](/docs/agent/options.html#_raft_protocol) setting in
the Agent configuration must be set to 3 or higher on all servers. In Consul
0.8 this setting defaults to 2; in Consul 1.0 it will default to 3. For more
information, see the [Version Upgrade section](/docs/upgrade-specific.html#raft_protocol)
on Raft Protocol versions.

In this guide we will learn more about Autopilot's features.

* Dead server cleanup
* Server Stabilization
* Redundancy zone tags
* Upgrade migration

Finally, we will review how to ensure Autopilot is healthy.

Note, in this guide we are using  examples from a Consul 1.4 cluster, we
are starting with Autopilot enabled by default.

## Default Configuration

The configuration of Autopilot is loaded by the leader from the agent's
[Autopilot settings](/docs/agent/options.html#autopilot) when initially
bootstrapping the cluster. Since Autopilot and it's features are already
enabled, we only need to update the configuration to disable them. The
following are the defaults.

```
{
    "cleanup_dead_servers": true,
    "last_contact_threshold": "200ms",
    "max_trailing_logs": 250,
    "server_stabilization_time": "10s",
    "redundancy_zone_tag": "",
    "disable_upgrade_migration": false,
    "upgrade_version_tag": ""
}
```

All Consul servers should have Autopilot and its features either enabled
or disabled to ensure consistency accross servers in case of a failure. Additionally,
Autopilot must be enabled to use any of the features, but the features themselves
can be configured independently. Meaning you can enable or disable any of the features
separately, at any time.

After bootstrapping, the configuration can be viewed or modified either via the
[`operator autopilot`](/docs/commands/operator/autopilot.html) subcommand or the
[`/v1/operator/autopilot/configuration`](/api/operator.html#autopilot-configuration)
HTTP endpoint.

```
$ consul operator autopilot get-config
CleanupDeadServers = true
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 10s
RedundancyZoneTag = ""
DisableUpgradeMigration = false
UpgradeVersionTag = ""
```

In the example above, we used the `operator autopilot get-config` subcommand to check
the autopilot configuration. You can see we still have all the defaults.

## Dead Server Cleanup

If Autopilot is disabled, it will take 72 hours for dead servers to be automatically reaped
or an operator had to script a `consul force-leave`. If another server failure occurred
it could jeopardize the quorum, even if the failed Consul server had been automatically
replaced. Autopilot helps prevent these kinds of outages by quickly removing failed
servers as soon as a replacement Consul server comes online. When servers are removed
by the cleanup process they will enter the "left" state.

With Autopilot's dead server cleanup enabled, dead servers will periodically be
cleaned up and removed from the Raft peer set to prevent them from interfering with
the quorum size and leader elections. The cleanup process will also be automatically
triggered whenever a new server is successfully added to the cluster.

To update the dead server cleanup feature use `consul operator autopilot set-config`
with the `-cleanup-dead-servers` flag.

```sh
$ consul operator autopilot set-config -cleanup-dead-servers=false
Configuration updated!

$ consul operator autopilot get-config
CleanupDeadServers = false
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 10s
RedundancyZoneTag = ""
DisableUpgradeMigration = false
UpgradeVersionTag = ""
```

We have disabled dead server cleanup, but sill have all the other Autopilot defaults.

## Server Stabilization

When a new server is added to the cluster, there is a waiting period where it
must be healthy and stable for a certain amount of time before being promoted
to a full, voting member. This can be configured via the `ServerStabilizationTime`
setting.

```sh
consul operator autopilot set-config -server-stabilization-time=5s
Configuration updated!

$ consul operator autopilot get-config
CleanupDeadServers = false
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 5s
RedundancyZoneTag = ""
DisableUpgradeMigration = false
UpgradeVersionTag = ""
```

Now we have disabled dead server cleanup and set the server stabilization time to 5 seconds.
When a new server is added to our cluster, it will only need to be healthy and stable for
5 seconds.

## Redundancy Zones

Prior to Autopilot, it was difficult to deploy servers in a way that took advantage of
isolated failure domains such as AWS Availability Zones; users would be forced to either
have an overly-large quorum (2-3 nodes per AZ) or give up redundancy within an AZ by
deploying just one server in each.

If the `RedundancyZoneTag` setting is set, Consul will use its value to look for a
zone in each server's specified [`-node-meta`](/docs/agent/options.html#_node_meta)
tag. For example, if `RedundancyZoneTag` is set to `zone`, and `-node-meta zone:east1a`
is used when starting a server, that server's redundancy zone will be `east1a`.

```
$ consul operator autopilot set-config -redundancy-zone-tag=uswest1
Configuration updated!

$ consul operator autopilot get-config
CleanupDeadServers = false
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 5s
RedundancyZoneTag = "uswest1"
DisableUpgradeMigration = false
UpgradeVersionTag = ""
```

For our Autopilot features, we now have disabled dead server cleanup, server stabilization time to 5 seconds, and
the redundancy zone tag is uswest1.

Consul will then use these values to partition the servers by redundancy zone, and will
aim to keep one voting server per zone. Extra servers in each zone will stay as non-voters
on standby to be promoted if the active voter leaves or dies.

## Upgrade Migrations

Autopilot in Consul *Enterprise* supports upgrade migrations by default. To disable this
functionality, set `DisableUpgradeMigration` to true.

```sh
$ consul operator autopilot set-config -disable-upgrade-migration=true
Configuration updated!

$ consul operator autopilot get-config
CleanupDeadServers = false
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 5s
RedundancyZoneTag = "uswest1"
DisableUpgradeMigration = true
UpgradeVersionTag = ""
```

With upgrade migration enabled, when a new server is added and Autopilot detects that
its Consul version is newer than that of the existing servers, Autopilot will avoid
promoting the new server until enough newer-versioned servers have been added to the
cluster. When the count of new servers equals or exceeds that of the old servers,
Autopilot will begin promoting the new servers to voters and demoting the old servers.
After this is finished, the old servers can be safely removed from the cluster.

To check the consul version of the servers, you can either use the [autopilot health]
(/api/operator.html#autopilot-health) endpoint or the `consul members`
command.

```
$ consul members
Node   Address         Status  Type    Build  Protocol  DC   Segment
node1  127.0.0.1:8301  alive   server  1.4.0  2         dc1   <all>
node2  127.0.0.1:8703  alive   server  1.4.0  2         dc1   <all>
node3  127.0.0.1:8803  alive   server  1.4.0  2         dc1   <all>
node4  127.0.0.1:8203  alive   server  1.3.0  2         dc1   <all>
```

### Migrations Without a Consul Version Change

The `UpgradeVersionTag` can be used to override the version information used during
a migration, so that the migration logic can be used for updating the cluster when
changing configuration.

If the `UpgradeVersionTag` setting is set, Consul will use its value to look for a
version in each server's specified [`-node-meta`](/docs/agent/options.html#_node_meta)
tag. For example, if `UpgradeVersionTag` is set to `build`, and `-node-meta build:0.0.2`
is used when starting a server, that server's version will be `0.0.2` when considered in
a migration. The upgrade logic will follow semantic versioning and the version string
must be in the form of either `X`, `X.Y`, or `X.Y.Z`.

```sh
$ consul operator autopilot set-config -upgrade-version-tag=1.4.0
Configuration updated!

$ consul operator autopilot get-config
CleanupDeadServers = false
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 5s
RedundancyZoneTag = "uswest1"
DisableUpgradeMigration = true
UpgradeVersionTag = "1.4.0"
```

## Server Health Checking

An internal health check runs on the leader to track the stability of servers.
<br>A server is considered healthy if all of the following conditions are true.

- It has a SerfHealth status of 'Alive'.
- The time since its last contact with the current leader is below
`LastContactThreshold`.
- Its latest Raft term matches the leader's term.
- The number of Raft log entries it trails the leader by does not exceed
`MaxTrailingLogs`.

The status of these health checks can be viewed through the [`/v1/operator/autopilot/health`]
(/api/operator.html#autopilot-health) HTTP endpoint, with a top level
`Healthy` field indicating the overall status of the cluster:

```
$ curl localhost:8500/v1/operator/autopilot/health
{
    "Healthy": true,
    "FailureTolerance": 0,
    "Servers": [
        {
            "ID": "e349749b-3303-3ddf-959c-b5885a0e1f6e",
            "Name": "node1",
            "Address": "127.0.0.1:8300",
            "SerfStatus": "alive",
            "Version": "0.8.0",
            "Leader": true,
            "LastContact": "0s",
            "LastTerm": 2,
            "LastIndex": 10,
            "Healthy": true,
            "Voter": true,
            "StableSince": "2017-03-28T18:28:52Z"
        },
        {
            "ID": "e35bde83-4e9c-434f-a6ef-453f44ee21ea",
            "Name": "node2",
            "Address": "127.0.0.1:8705",
            "SerfStatus": "alive",
            "Version": "0.8.0",
            "Leader": false,
            "LastContact": "35.371007ms",
            "LastTerm": 2,
            "LastIndex": 10,
            "Healthy": true,
            "Voter": false,
            "StableSince": "2017-03-28T18:29:10Z"
        }
    ]
}
```

## Summary

In this guide we configured most of the Autopilot features; dead server cleanup, server
stabilization, redundancy zone tags, upgrade migration, and upgrade version tag.

To learn more about the Autopilot settings we did not configure,
[last_contact_threshold](https://www.consul.io/docs/agent/options.html#last_contact_threshold)
and [max_trailing_logs](https://www.consul.io/docs/agent/options.html#max_trailing_logs),
either read the agent configuration documentation or use the help flag with the
operator autopilot `consul operator autopilot set-config -h`.
