---
layout: "docs"
page_title: "Commands"
sidebar_current: "docs-commands"
description: |-
  Consul is controlled via a very easy to use command-line interface (CLI). Consul is only a single command-line application: `consul`. This application then takes a subcommand such as agent or members. The complete list of subcommands is in the navigation to the left.
---

# Consul Commands (CLI)

Consul is controlled via a very easy to use command-line interface (CLI).
Consul is only a single command-line application: `consul`. This application
then takes a subcommand such as "agent" or "members". The complete list of
subcommands is in the navigation to the left.

The `consul` CLI is a well-behaved command line application. In erroneous
cases, a non-zero exit status will be returned. It also responds to `-h` and `--help`
as you'd most likely expect. And some commands that expect input accept
"-" as a parameter to tell Consul to read the input from stdin.

To view a list of the available commands at any time, just run `consul` with
no arguments:

```text
$ consul
usage: consul [--version] [--help] <command> [<args>]

Available commands are:
    agent          Runs a Consul agent
    catalog        Interact with the catalog
    event          Fire a new event
    exec           Executes a command on Consul nodes
    force-leave    Forces a member of the cluster to enter the "left" state
    info           Provides debugging information for operators.
    join           Tell Consul agent to join cluster
    keygen         Generates a new encryption key
    keyring        Manages gossip layer encryption keys
    kv             Interact with the key-value store
    leave          Gracefully leaves the Consul cluster and shuts down
    lock           Execute a command holding a lock
    maint          Controls node or service maintenance mode
    members        Lists the members of a Consul cluster
    monitor        Stream logs from a Consul agent
    operator       Provides cluster-level tools for Consul operators
    reload         Triggers the agent to reload configuration files
    rtt            Estimates network round trip time between nodes
    snapshot       Saves, restores and inspects snapshots of Consul server state
    validate       Validate config files/directories
    version        Prints the Consul version
    watch          Watch for changes in Consul
```

To get help for any specific command, pass the `-h` flag to the relevant
subcommand. For example, to see help about the `join` subcommand:

```text
$ consul join -h
Usage: consul join [options] address ...

  Tells a running Consul agent (with "consul agent") to join the cluster
  by specifying at least one existing member.

HTTP API Options

  -http-addr=<address>
     The `address` and port of the Consul HTTP agent. The value can be
     an IP address or DNS address, but it must also include the port.
     This can also be specified via the CONSUL_HTTP_ADDR environment
     variable. The default value is http://127.0.0.1:8500. The scheme
     can also be set to HTTPS by setting the environment variable
     CONSUL_HTTP_SSL=true.

  -token=<value>
     ACL token to use in the request. This can also be specified via the
     CONSUL_HTTP_TOKEN environment variable. If unspecified, the query
     will default to the token of the Consul agent at the HTTP address.

Command Options

  -wan
     Joins a server to another server in the WAN pool.
```

## Autocompletion

The `consul` command features opt-in subcommand autocompletion that you can
enable for your shell with `consul -autocomplete-install`. After doing so,
you can invoke a new shell and use the feature.

For example, assume a tab is typed at the end of each prompt line:

```
$ consul e
event  exec

$ consul r
reload  rtt

$ consul operator raft
list-peers   remove-peer
```

## Environment Variables

In addition to CLI flags, Consul reads environment variables for behavior
defaults. CLI flags always take precedence over environment variables, but it
is often helpful to use environment variables to configure the Consul agent,
particularly with configuration management and init systems.

These environment variables and their purpose are described below:

## `CONSUL_HTTP_ADDR`

This is the HTTP API address to the *local* Consul agent
(not the remote server) specified as a URI:

```
CONSUL_HTTP_ADDR=127.0.0.1:8500
```

or as a Unix socket path:

```
CONSUL_HTTP_ADDR=unix://var/run/consul_http.sock
```

### `CONSUL_HTTP_TOKEN`

This is the API access token required when access control lists (ACLs)
are enabled, for example:

```
CONSUL_HTTP_TOKEN=aba7cbe5-879b-999a-07cc-2efd9ac0ffe
```

### `CONSUL_HTTP_AUTH`

This specifies HTTP Basic access credentials as a username:password pair:

```
CONSUL_HTTP_AUTH=operations:JPIMCmhDHzTukgO6
```

### `CONSUL_HTTP_SSL`

This is a boolean value (default is false) that enables the HTTPS URI
scheme and SSL connections to the HTTP API:

```
CONSUL_HTTP_SSL=true
```

### `CONSUL_HTTP_SSL_VERIFY`

This is a boolean value (default true) to specify SSL certificate verification; setting this value to `false` is not recommended for production use. Example
for development purposes:

```
CONSUL_HTTP_SSL_VERIFY=false
```

### `CONSUL_CACERT`

Path to a CA file to use for TLS when communicating with Consul.

```
CONSUL_CACERT=ca.crt
```

### `CONSUL_CAPATH`

Path to a directory of CA certificates to use for TLS when communicating with Consul.

```
CONSUL_CAPATH=ca_certs/
```

### `CONSUL_CLIENT_CERT`

Path to a client cert file to use for TLS when `verify_incoming` is enabled.

```
CONSUL_CLIENT_CERT=client.crt
```

### `CONSUL_CLIENT_KEY`

Path to a client key file to use for TLS when `verify_incoming` is enabled.

```
CONSUL_CLIENT_KEY=client.key
```

### `CONSUL_TLS_SERVER_NAME`

The server name to use as the SNI host when connecting via TLS.

```
CONSUL_TLS_SERVER_NAME=consulserver.domain
```
