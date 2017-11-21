---
layout: "docs"
page_title: "Commands: Catalog"
sidebar_current: "docs-commands-catalog"
---

# Consul Catalog

Command: `consul catalog`

The `catalog` command is used to interact with Consul's catalog via the command
line. It exposes top-level commands for reading and filtering data from the
registry.

The catalog is also accessible via the [HTTP API](/api/catalog.html).

## Basic Examples

List all datacenters:

```text
$ consul catalog datacenters
dc1
dc2
dc3
```

List all nodes:

```text
$ consul catalog nodes
Node       ID        Address    DC
worker-01  1b662d97  10.4.5.31  dc1
```

List all nodes which provide a particular service:

```text
$ consul catalog nodes -service=redis
Node       ID        Address     DC
worker-01  1b662d97  10.4.5.31   dc1
worker-02  d407a592  10.4.4.158  dc1
```

List all services:

```text
$ consul catalog services
consul
postgresql
redis
```

List all services on a node:

```text
$ consul catalog services -node=worker-01
consul
postgres
```

For more examples, ask for subcommand help or view the subcommand documentation
by clicking on one of the links in the sidebar.

## Usage

Usage: `consul catalog <subcommand>`

For the exact documentation for your Consul version, run `consul catalog -h` to
view the complete list of subcommands.

```text
Usage: consul catalog <subcommand> [options] [args]

  # ...

Subcommands:
    datacenters    Lists all known datacenters for this agent
    nodes          Lists all nodes in the given datacenter
    services       Lists all registered services in a datacenter
```

For more information, examples, and usage about a subcommand, click on the name
of the subcommand in the sidebar or one of the links below:
