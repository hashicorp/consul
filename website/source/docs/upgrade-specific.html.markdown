---
layout: "docs"
page_title: "Upgrading Specific Versions"
sidebar_current: "docs-upgrading-specific"
description: |-
  Specific versions of Consul may have additional information about the upgrade process beyond the standard flow.
---

# Upgrading Specific Versions

The [upgrading page](/docs/upgrading.html) covers the details of doing
a standard upgrade. However, specific versions of Consul may have more
details provided for their upgrades as a result of new features or changed
behavior. This page is used to document those details seperately from the
standard upgrade flow.

## Consul 0.5.1

Consul version 0.5.1 uses a different backend store for persisting the Raft
log. Because of this change, a data migration is necessary to move the log
entries out of LMDB and into the newer backend, BoltDB.

Consul version 0.5.1 makes this transition seamless and easy. As a user, there
are no special steps you need to take. When Consul 0.5.1 starts, it checks
for presence of the legacy LMDB data files, and migrates them automatically
if any are found. You will see a log emitted when Raft data is migrated, like
this:

```
==> Successfully migrated raft data in 5.839642ms
```

The automatic upgrade will only exist in Consul 0.5.1. In later versions
(0.6.0+), the migration code will not be included in the Consul binary. It
is still possible to upgrade directly from pre-0.5.1 versions by using the
consul-migrate utility, which is available on the
[Consul Tools page](/downloads_tools.html).

## Consul 0.5

Consul version 0.5 adds two features that complicate the upgrade process:

* ACL system includes service discovery and registration
* Internal use of tombstones to fix behavior of blocking queries
  in certain edge cases.

Users of the ACL system need to be aware that deploying Consul 0.5 will
cause service registration to be enforced. This means if an agent
attempts to register a service without proper privileges it will be denied.
If the `acl_default_policy` is "allow" then clients will continue to
work without an updated policy. If the policy is "deny", then all clients
will begin to have their registration rejected causing issues.

To avoid this situation, all the ACL policies should be updated to
add something like this:

    # Enable all services to be registered
    service "" {
        policy = "write"
    }

This will set the service policy to `write` level for all services.
The blank service name is the catch-all value. A more specific service
can also be specified:

    # Enable only the API service to be registered
    service "api" {
        policy = "write"
    }

The ACL policy can be updated while running 0.4, and enforcement will
being with the upgrade to 0.5. The policy updates will ensure the
availability of the cluster.

The second major change is the new internal command used for tombstones.
The details of the change are not important, however to function the leader
node will replicate a new command to its followers. Consul is designed
defensively, and when a command that is not recognized is received, the
server will panic. This is a purposeful design decision to avoid the possibility
of data loss, inconsistensies, or security issues caused by future incompatibility.

In practice, this means if a Consul 0.5 node is the leader, all of its
followers must also be running 0.5. There are a number of ways to do this
to ensure cluster availability:

* Add new 0.5 nodes, then remove the old servers. This will add the new
  nodes as followers, and once the old servers are removed, one of the
  0.5 nodes will become leader.

* Upgrade the followers first, then the leader last. Using `consul info`,
  you can determine which nodes are followers. Do an in-place upgrade
  on them first, and finally upgrade the leader last.

* Upgrade them in any order, but ensure all are done within 15 minutes.
  Even if the leader is upgraded to 0.5 first, as long as all of the followers
  are running 0.5 within 15 minutes there will be no issues.

Finally, even if any of the methods above are not possible or the process
fails for some reason, it is not fatal. The older version of the server
will simply panic and stop. At that point, you can upgrade to the new version
and restart the agent. There will be no data loss and the cluster will
resume operations.

