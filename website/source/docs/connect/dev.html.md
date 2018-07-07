---
layout: "docs"
page_title: "Connect - Development and Debugging"
sidebar_current: "docs-connect-dev"
description: |-
  It is often necessary to connect to a service for development or debugging. If a service only exposes a Connect listener, then we need a way to establish a mutual TLS connection to the service. The `consul connect proxy` command can be used for this task on any machine with access to a Consul agent (local or remote).
---

# Developing and Debugging Connect Services

It is often necessary to connect to a service for development or debugging.
If a service only exposes a Connect listener, then we need a way to establish
a mutual TLS connection to the service. The
[`consul connect proxy` command](/docs/commands/connect/proxy.html) can be used
for this task on any machine with access to a Consul agent (local or remote).

Restricting access to services only via Connect ensures that the only way to
connect to a service is through valid authorization of the
[intentions](/docs/connect/intentions.html). This can extend to developers
and operators, too.

## Connecting to Connect-only Services

As an example, let's assume that we have a PostgreSQL database running that
we want to connect to via `psql`, but the only non-loopback listener is
via Connect. Let's also assume that we have an ACL token to identify as
`operator-mitchellh`. We can start a local proxy:

```sh
$ consul connect proxy \
  -service operator-mitchellh \
  -upstream postgresql:8181
```

This works because the source `-service` does not need to be registered
in the local Consul catalog. However, to retrieve a valid identifying
certificate, the ACL token must have `service:write` permissions. This
can be used as a sort of "virtual service" to represent people, too. In
the example above, the proxy is identifying as `operator-mitchellh`.

With the proxy running, we can now use `psql` like normal:

```
$ psql -h 127.0.0.1 -p 8181 -U mitchellh mydb
>
```

This `psql` session is now happening through our local proxy via an
authorized mutual TLS connection to the PostgreSQL service in our Consul
catalog.

### Masquerading as a Service

You can also easily masquerade as any source service by setting the
`-service` value to any service. Note that the proper ACL permissions are
required to perform this task.

For example, if you have an ACL token that allows `service:write` for
`web` and you want to connect to the `postgresql` service as "web", you
can start a proxy like so:

```sh
$ consul connect proxy \
  -service web \
  -upstream postgresql:8181
```

