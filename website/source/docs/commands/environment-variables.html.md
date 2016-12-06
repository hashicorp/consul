---
layout: "docs"
page_title: "Environment Variables"
sidebar_current: "docs-commands-environment-variables"
description: |-
  Consul's behavior can be modified by certain environment variables.
---

# Environment variables

In addition to CLI flags, Consul reads environment variables for behavior
defaults. CLI flags always take precedence over environment variables, but it
is often helpful to use environment variables to configure the Consul agent,
particularly with configuration management and init systems.

The following table describes these variables:

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

## `CONSUL_HTTP_TOKEN`

This is the API access token required when access control lists (ACLs)
are enabled, for example:

```
CONSUL_HTTP_TOKEN=aba7cbe5-879b-999a-07cc-2efd9ac0ffe
```

## `CONSUL_HTTP_AUTH`

This specifies HTTP Basic access credentials as a username:password pair:

```
CONSUL_HTTP_AUTH=operations:JPIMCmhDHzTukgO6
```

## `CONSUL_HTTP_SSL`

This is a boolean value (default is false) that enables the HTTPS URI
scheme and SSL connections to the HTTP API:

```
CONSUL_HTTP_SSL=true
```

## `CONSUL_HTTP_SSL_VERIFY`

This is a boolean value (default true) to specify SSL certificate verification; setting this value to `false` is not recommended for production use. Example
for development purposes:

```
CONSUL_HTTP_SSL_VERIFY=false
```

## `CONSUL_RPC_ADDR`

This is the RPC interface address for the local agent specified as a URI:

```
CONSUL_RPC_ADDR=127.0.0.1:8300
```

or as a Unix socket path:

```
CONSUL_RPC_ADDR=unix://var/run/consul_rpc.sock
```
