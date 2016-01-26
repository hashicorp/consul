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
behavior. This page is used to document those details separately from the
standard upgrade flow.

## Consul 0.6

Consul version 0.6 is a very large release with many enhancements and
optimizations. Changes to be aware of during an upgrade are categorized below.

#### Data store changes

Consul changed the format used to store data on the server nodes in version 0.5
(see 0.5.1 notes below for details). Previously, Consul would automatically
detect data directories using the old LMDB format, and convert them to the newer
BoltDB format. This automatic upgrade has been removed for Consul 0.6, and
instead a safeguard has been put in place which will prevent Consul from booting
if the old directory format is detected.

It is still possible to migrate from a 0.5.x version of Consul to 0.6+ using the
[consul-migrate](https://github.com/hashicorp/consul-migrate) CLI utility. This
is the same tool that was previously embedded into Consul. See the
[releases](https://github.com/hashicorp/consul-migrate/releases) page for
downloadable versions of the tool.

Also, in this release Consul switched from LMDB to a fully in-memory database for
the state store. Because LMDB is a disk-based backing store, it was able to store
more data than could fit in RAM in some cases (though this is not a recommended
configuration for Consul). If you have an extremely large data set that won't fit
into RAM, you may encounter issues upgrading to Consul 0.6.0 and later. Consul
should be provisioned with physical memory approximately 2X the data set size to
allow for bursty allocations and subsequent garbage collection.

#### ACL Enhancements

Consul 0.6 introduces enhancements to the ACL system which may require special
handling:

* Service ACLs are enforced during service discovery (REST + DNS)

Previously, service discovery was wide open, and any client could query
information about any service without providing a token. Consul now requires
read-level access at a minimum when ACLs are enabled to return service
information over the REST or DNS interfaces. If clients depend on an open
service discovery system, then the following should be added to all ACL tokens
which require it:

    # Enable discovery of all services
    service "" {
        policy = "read"
    }

Note that the agent's [`acl_token`](/docs/agent/options.html#acl_token) is used
when the DNS interface is queried, so be sure that token has sufficient
privileges to return the DNS records you expect to retrieve from it.

* Event and keyring ACLs

Similar to service discovery, the new event and keyring ACLs will block access
to these operations if the `acl_default_policy` is set to `deny`. If clients depend
on open access to these, then the following should be added to all ACL tokens which
require them:

    event "" {
      policy = "write"
    }

    keyring = "write"

Unfortunately, these are new ACLs for Consul 0.6, so they must be added after the
upgrade is complete.

#### Prepared Queries

Prepared queries introduce a new Raft log entry type that isn't supported on older
versions of Consul. It's important to not use the prepared query features of Consul
until all servers in a cluster have been upgraded to version 0.6.0.

#### Single Private IP Enforcement

Consul will refuse to start if there are multiple private IPs available, so
if this is the case you will need to configure Consul's advertise or bind addresses
before upgrading.

#### New Web UI File Layout

The release .zip file for Consul's web UI no longer contains a `dist` sub-folder;
everything has been moved up one level. If you have any automated scripts that
expect the old layout you may need to update them.

## Consul 0.5.1

Consul version 0.5.1 uses a different backend store for persisting the Raft
log. Because of this change, a data migration is necessary to move the log
entries out of LMDB and into the newer backend, BoltDB.

Consul version 0.5.1+ makes this transition seamless and easy. As a user, there
are no special steps you need to take. When Consul starts, it checks
for presence of the legacy LMDB data files, and migrates them automatically
if any are found. You will see a log emitted when Raft data is migrated, like
this:

```
==> Successfully migrated raft data in 5.839642ms
```

This automatic upgrade will only exist in Consul 0.5.1+ and it will
be removed starting with Consul 0.6.0+. It will still be possible to upgrade directly 
from pre-0.5.1 versions by using the consul-migrate utility, which is available on the
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

