---
layout: "docs"
page_title: "Cluster Backups"
sidebar_current: "docs-guides-backups"
description: |-
  Consul provide the snapshot tool for backing up and restoring data. In this guide you will learn how to use both.
---

# Cluster Backups

Creating server backups is an important step in production deployments. Backups provide a mechanism for the server to recover from an outage (network loss, operator error, or a corrupted data directory). All agents write to the `-data-dir` before commit. This directory persists the local agent’s state and - in the case of servers -  it also holds the Raft information.

Consul provides the [snapshot](https://consul.io/docs/commands/snapshot.html) command which can be run using the CLI or the API. The `snapshot` command saves a point-in-time snapshot of the state of the Consul servers which includes:

* KV entries 
* the service catalog
* prepared queries
* sessions 
* ACLs

With [Consul Enterprise](/docs/commands/snapshot/agent.html), the `snapshot agent` command runs periodically and writes to local or remote storage (such as Amazon S3).

By default, all snapshots are taken using `consistent` mode where requests are forwarded to the leader which verifies that it is still in power before taking the snapshot. Snapshots will not be saved if the cluster is degraded or if no leader is available. To reduce the burden on the leader, it is possible to [run the snapshot](/docs/commands/snapshot/save.html) on any non-leader server using `stale` consistency mode.

This spreads the load across nodes at the possible expense of losing full consistency guarantees. Typically this means that a very small number of recent writes may not be included. The omitted writes are typically limited to data written in the last `100ms` or less from the recovery point. This is usually suitable for disaster recovery. However, the system can’t guarantee how stale this may be if executed against a partitioned server.

## Create Your First Backup

The `snapshot save` command for backing up the cluster state has many configuration options. In a production environment, you will want to configure ACL tokens and client certificates for security. The configuration options also allow you to specify the datacenter and server to collect the backup data from. Below are several examples. 
 
First, we will run the basic snapshot command with the all the defaults, including `consistent` mode.

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

Next, let’s collect the cluster data from a non-leader by specifying stale mode.

```sh
$ consul snapshot save -stale backup.snap
Saved and verified snapshot to index 2276
```

Once ACLs and agent certificates are configured, they can be passed in with the following flags.

```sh
$ consul snapshot save -stale -token=<ead6787-23thbd-666789> -ca-file=</path/to/file> backup.snap
Saved and verified snapshot to index 2287
```

Alternatively, you can use environment variables to say the ACL token and CA cert. 

For production use, the  `snapshot save` command or [API](https://www.consul.io/api/snapshot.html) should be scripted and run frequently. In addition to frequently backing up the cluster state, there are several use cases when you would also want to manually execute `snapshot save`. First, it should be best practice to backup the cluster before upgrading. If the upgrade does not go according to plan, this will ensure you have the latest data. Second, if the cluster loses quorum it may be beneficial to save the state before the non-leaders become divergent. Finally, you can manually snapshot the cluster for use of bootstrapping a new cluster. 

Operationally, the backup process does not need to be executed on every server. Additionally, you can use the configuration options to save the backups to a mounted filesystem. The mounted filesystem can even be cloud storage, such as Amazon S3. The enterprise command `snapshot agent` automates this process.

## Restore from Backup 

Running the `restore` process should be straightforward. However, there are a couple actions you can take to ensure the process goes smoothly. First, make sure the cluster you are restoring is stable. Second, you will only need to run the process once, on the leader. The cluster gossip protocol will ensure the data is propagated to the non-leaders.  

The basic restore command. 

```sh
$ consul snapshot restore backup.snap
Restored snapshot
```
Like the `save` subcommand, restore has many configuration options. In production, you would again want to use ACLs and certificates for security. 

## Summary 

In this guide, we learned about the `snapshot save` and `snapshot restore` commands. If you are testing the backup and restore process, you can add an extra dummy value to Consul KV. Another indicator that the backup was saved correctly is the size of the backup artifact. 

