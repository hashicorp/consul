---
layout: "docs"
page_title: "Autopilot"
sidebar_current: "docs-guides-autopilot"
description: |-
  This guide covers how to configure and use Autopilot features.
---

# Autopilot

Autopilot is a set of new features added in Consul 0.8 to allow for automatic
operator-friendly management of Consul servers. It includes cleanup of dead
servers, monitoring the state of the Raft cluster, and stable server introduction.

To enable Autopilot features (with the exception of dead server cleanup),
the [`raft_protocol`](/docs/agent/options.html#_raft_protocol) setting in
the Agent configuration must be set to 3 or higher on all servers. In Consul
0.8 this setting defaults to 2; in Consul 0.9 it will default to 3. For more
information, see the [Version Upgrade section](/docs/upgrade-specific.html#raft_protocol)
on Raft Protocol versions.

## Configuration

The configuration of Autopilot is loaded by the leader from the agent's
[`autopilot`](/docs/agent/options.html#autopilot) settings when initially
bootstrapping the cluster. After bootstrapping, the configuration can
be viewed or modified either via the [`operator autopilot`]
(/docs/commands/operator/autopilot.html) subcommand or the
[`/v1/operator/autopilot/configuration`](/docs/agent/http/operator.html#autopilot-configuration)
HTTP endpoint:

```
$ consul operator autopilot get-config
CleanupDeadServers = true
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 10s

$ consul operator autopilot set-config -cleanup-dead-servers=false
Configuration updated!

$ consul operator autopilot get-config
CleanupDeadServers = false
LastContactThreshold = 200ms
MaxTrailingLogs = 250
ServerStabilizationTime = 10s
```

## Dead Server Cleanup

Dead servers will periodically be cleaned up and removed from the Raft peer
set, to prevent them from interfering with the quorum size and leader elections.
This cleanup will also happen whenever a new server is successfully added to the
cluster.

Prior to Autopilot, it would take 72 hours for dead servers to be automatically reaped,
or operators had to script a `consul force-leave`. If another server failure occurred,
it could jeopardize the quorum, even if the failed Consul server had been automatically
replaced. Autopilot helps prevent these kinds of outages by quickly removing failed
servers as soon as a replacement Consul server comes online.

This option can be disabled by running `consul operator autopilot set-config`
with the `-cleanup-dead-servers=false` option.

## Server Health Checking

An internal health check runs on the leader to track the stability of servers.
</br>A server is considered healthy if:

- It has a SerfHealth status of 'Alive'
- The time since its last contact with the current leader is below
`LastContactThreshold`
- Its latest Raft term matches the leader's term
- The number of Raft log entries it trails the leader by does not exceed
`MaxTrailingLogs`

The status of these health checks can be viewed through the [`/v1/operator/autopilot/health`]
(/docs/agent/http/operator.html#autopilot-health) HTTP endpoint, with a top level
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
            "SerfStatus": "alive",
            "LastContact": "0s",
            "LastTerm": 3,
            "LastIndex": 23,
            "Healthy": true,
            "StableSince": "2017-03-10T22:01:14Z"
        },
        {
            "ID": "099061c7-ea74-42d5-be04-a0ad74caaaf5",
            "Name": "node2",
            "SerfStatus": "alive",
            "LastContact": "53.279635ms",
            "LastTerm": 3,
            "LastIndex": 23,
            "Healthy": true,
            "StableSince": "2017-03-10T22:03:26Z"
        }
    ]
}
```

## Stable Server Introduction

When a new server is added to the cluster, there is a waiting period where it
must be healthy and stable for a certain amount of time before being promoted
to a full, voting member. This can be configured via the `ServerStabilizationTime`
setting.
