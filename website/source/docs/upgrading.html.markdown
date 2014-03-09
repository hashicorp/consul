---
layout: "docs"
page_title: "Upgrading Consul"
sidebar_current: "docs-upgrading-upgrading"
---

# Upgrading Consul

Consul is meant to be a long-running agent on any nodes participating in a
Consul cluster. These nodes consistently communicate with each other. As such,
protocol level compatibility and ease of upgrades is an important thing to
keep in mind when using Consul.

This page documents how to upgrade Consul when a new version is released.

## Upgrading Consul

In short, upgrading Consul is a short series of easy steps. For the steps
below, assume you're running version A of Consul, and then version B comes out.

1. On each node, install version B of Consul.

2. Shut down version A, and start version B with the `-protocol=PREVIOUS`
   flag, where "PREVIOUS" is the protocol version of version A (which can
   be discovered by running `consul -v` or `consul members`).

3. Once all nodes are running version B, go through every node and restart
   the version B agent _without_ the `-protocol` flag.

4. Done! You're now running the latest Consul agent speaking the latest protocol.
   You can verify this is the case by running `consul members` to
   make sure all members are speaking the same, latest protocol version.

The key to making this work is the [protocol compatibility](/docs/compatibility.html)
of Consul. The protocol version system is discussed below.

## Protocol Versions

By default, Consul agents speak the latest protocol they can. However, each
new version of Consul is also able to speak the previous protocol, if there
were any protocol changes.

You can see what protocol versions your version of Consul understands by
running `consul -v`. You'll see output similar to that below:

```
$ consul -v
Consul v0.1.0
Consul Protocol: 1 (Understands back to: 1)
```

This says the version of Consul as well as the latest protocol version (1,
in this case). It also says the earliest protocol version that this Consul
agent can understand (0, in this case).

By specifying the `-protocol` flag on `consul agent`, you can tell the
Consul agent to speak any protocol version that it can understand. This
only specifies the protocol version to _speak_. Every Consul agent can
always understand the entire range of protocol versions it claims to
on `consul -v`.

<div class="alert alert-block alert-warning">
<strong>By running a previous protocol version</strong>, some features
of Consul, especially newer features, may not be available. If this is the
case, Consul will typically warn you. In general, you should always upgrade
your cluster so that you can run the latest protocol version.
</div>
