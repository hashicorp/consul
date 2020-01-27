---
layout: "docs"
page_title: "Datacenter Backups"
sidebar_current: "docs-guides-backups"
description: |-
  Consul provide the snapshot tool for backing up and restoring data. In this guide you will learn how to use both.
---

# Datacenter Backups

Creating datacenter backups is an important step in production deployments. Backups provide a mechanism for the Consul server to recover from an outage (network loss, operator error, or a corrupted data directory). All servers write to the `-data-dir` before commit on write requests. The same directory is used on client agents to persist local state too, but this is not critical and can be rebuilt when recreating an agent. Local client state is not backed up in this guide and doesn't need to be in general, only the server's Raft store state.

Consul provides the [snapshot](https://consul.io/docs/commands/snapshot.html) command which can be run using the CLI or the API. The `snapshot` command saves a point-in-time snapshot of the state of the Consul servers which includes, but is not limited to:

* KV entries 
* the service catalog
* prepared queries
* sessions 
* ACLs

With [Consul Enterprise](/docs/commands/snapshot/agent.html), the `snapshot agent` command runs periodically and writes to local or remote storage (such as Amazon S3).

By default, all snapshots are taken using `consistent` mode where requests are forwarded to the leader which verifies that it is still in power before taking the snapshot. Snapshots will not be saved if the datacenter is degraded or if no leader is available. To reduce the burden on the leader, it is possible to [run the snapshot](/docs/commands/snapshot/save.html) on any non-leader server using `stale` consistency mode.

This spreads the load across nodes at the possible expense of losing full consistency guarantees. Typically this means that a very small number of recent writes may not be included. The omitted writes are typically limited to data written in the last `100ms` or less from the recovery point. This is usually suitable for disaster recovery. However, the system can’t guarantee how stale this may be if executed against a partitioned server.

## Create Your First Backup

The `snapshot save` command for backing up the datacenter state has many configuration options. In a production environment, you will want to configure ACL tokens and client certificates for security. The configuration options also allow you to specify the datacenter and server to collect the backup data from. Below are several examples. 
 
First, we will run the basic snapshot command on one of our servers with the all the defaults, including `consistent` mode.

```sh
$ consul snapshot save backup.snap
Saved and verified snapshot to index 1176
```
The backup will be saved locally in the directory where we ran the command. 

You can view metadata about the backup with the `inspect` subcommand. 

```sh
$ consul snapshot inspect backup.snap
ID           2-1182-1542056499724
Size         4115
Index        1182
Term         2
Version      1
```

To understand each field review the inspect [documentation](https://www.consul.io/docs/commands/snapshot/inspect.html). Notably, the `Version` field does not correspond to the version of the data. Rather it is the snapshot format version. 

Next, let’s collect the datacenter data from a non-leader server by specifying stale mode.

```sh
$ consul snapshot save -stale backup.snap
Saved and verified snapshot to index 2276
```

Once ACLs and agent certificates are configured, they can be passed in as environtmennt variables or flags.

```sh
$ export CONSUL_HTTP_TOKEN=<your ACL token>
$ consul snapshot save -stale -ca-file=</path/to/file> backup.snap
Saved and verified snapshot to index 2287
```

In the above example, we set the token as an ENV and the ca-file with a command line flag. 

For production use, the  `snapshot save` command or [API](https://www.consul.io/api/snapshot.html) should be scripted and run frequently. In addition to frequently backing up the datacenter state, there are several use cases when you would also want to manually execute `snapshot save`. First, you should always backup the datacenter before upgrading. If the upgrade does not go according to plan it is often not possible to downgrade due to changes in the state store format. Restoring from a backup is the only option so taking one before the upgrade will ensure you have the latest data. Second, if the datacenter loses quorum it may be beneficial to save the state before the servers become divergent. Finally, you can manually snapshot a datacenter and use that to bootstrap a new datacenter with the same state. 

Operationally, the backup process does not need to be executed on every server. Additionally, you can use the configuration options to save the backups to a mounted filesystem. The mounted filesystem can even be cloud storage, such as Amazon S3. The enterprise command `snapshot agent` automates this process.

## Restore from Backup 

Running the `restore` process should be straightforward. However, there are a couple of actions you can take to ensure the process goes smoothly. First, make sure the datacenter you are restoring is stable and has a leader. You can see this using `consul operator raft list-peers` and checking server logs and telemetry for signs of leader elections or network issues. 

You will only need to run the process once, on the leader. The Raft consensus protocol ensures that all servers restore the same state.

```sh
$ consul snapshot restore backup.snap
Restored snapshot
```
Like the `save` subcommand, restore has many configuration options. In production, you would again want to use ACLs and certificates for security. 

## Summary 

In this guide, we learned about the `snapshot save` and `snapshot restore` commands. If you are testing the backup and restore process, you can add an extra dummy value to Consul KV. Another indicator that the backup was saved correctly is the size of the backup artifact. 

