---
layout: "docs"
page_title: "Commands: Snapshot"
sidebar_current: "docs-commands-snapshot"
---

# Consul Snapshot

Command: `consul snapshot`

The `snapshot` command has subcommands for saving and restoring the state of the
Consul servers for disaster recovery. These are atomic, point-in-time snapshots
which include key/value entries, service catalog, prepared queries, sessions, and
ACLs. This command is available in Consul 0.7.1 and later.

Snapshots are also accessible via the [HTTP API](/docs/agent/http/snapshot.html).

## Usage

Usage: `consul snapshot <subcommand>`

For the exact documentation for your Consul version, run `consul snapshot -h` to
view the complete list of subcommands.

```text
Usage: consul snapshot <subcommand> [options] [args]

  # ...

Subcommands:

    restore    Restores snapshot of Consul server state
    save       Saves snapshot of Consul server state
```

For more information, examples, and usage about a subcommand, click on the name
of the subcommand in the sidebar or one of the links below:

- [restore](/docs/commands/snapshot/restore.html)
- [save](/docs/commands/snapshot/save.html)

## Basic Examples

To create a snapshot and save it as a file called "backup.snap":

```text
$ consul snapshot save backup.snap
Saved and verified snapshot to index 8419
```

To restore a snapshot from a file called "backup.snap":

```text
$ consul snapshot restore backup.snap
Restored snapshot
```

For more examples, ask for subcommand help or view the subcommand documentation
by clicking on one of the links in the sidebar.
