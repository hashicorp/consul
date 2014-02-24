---
layout: "docs"
page_title: "RPC"
sidebar_current: "docs-agent-rpc"
---

# RPC Protocol

The Consul agent provides a complete RPC mechanism that can
be used to control the agent programmatically. This RPC
mechanism is the same one used by the CLI, but can be
used by other applications to easily leverage the power
of Consul without directly embedding. Additionally, it can
be used as a fast IPC mechanism to allow applications to
receive events immediately instead of using the fork/exec
model of event handlers.

## Implementation Details

The RPC protocol is implemented using [MsgPack](http://msgpack.org/)
over TCP. This choice is driven by the fact that all operating
systems support TCP, and MsgPack provides a fast serialization format
that is broadly available across languages.

All RPC requests have a request header, and some requests have
a request body. The request header looks like:

```
    {"Command": "Handshake", "Seq": 0}
```

All responses have a response header, and some may contain
a response body. The response header looks like:

```
    {"Seq": 0, "Error": ""}
```

The `Command` is used to specify what command the server should
run, and the `Seq` is used to track the request. Responses are
tagged with the same `Seq` as the request. This allows for some
concurrency on the server side, as requests are not purely FIFO.
Thus, the `Seq` value should not be re-used between commands.
All responses may be accompanied by an error.

Possible commands include:

* handshake - Used to initialize the connection, set the version
* force-leave - Removes a failed node from the cluster
* join - Requests Consul join another node
* members-lan - Returns the list of lan members
* members-wan - Returns the list of wan members
* monitor - Starts streaming logs over the connection
* stop - Stops streaming logs
* leave - Consul agent performs a graceful leave and shutdown
* stats - Provides various debugging statistics

Below each command is documented along with any request or
response body that is applicable.

### handshake

The handshake MUST be the first command that is sent, as it informs
the server which version the client is using.

The request header must be followed with a handshake body, like:

```
    {"Version": 1}
```

The body specifies the IPC version being used, however only version
1 is currently supported. This is to ensure backwards compatibility
in the future.

There is no special response body, but the client should wait for the
response and check for an error.

### force-leave

This command is used to remove failed nodes from a cluster. It takes
the following body:

```
    {"Node": "failed-node-name"}
```

There is no special response body.

### join

This command is used to join an existing cluster using a known node.
It takes the following body:

```
    {"Existing": ["192.168.0.1:6000", "192.168.0.2:6000"], "WAN": false}
```

The `Existing` nodes are each contacted, and `WAN` controls if we are adding a
WAN member or LAN member. LAN members are expected to be in the same datacenter,
and should be accessible at relatively low latencies. WAN members are expected to
be operating in different datacenters, with relatively high access latencies. It is
important that only agents running in "server" mode are able to join nodes over the
WAN.

The response body in addition to the header is returned. The body looks like:

```
    {"Num": 2}
```

The body returns the number of nodes successfully joined.

### members-lan

The members-lan command is used to return all the known lan members and associated
information. All agents will respond to this command.

There is no request body, but the response looks like:

```
    {"Members": [
        {
        "Name": "TestNode"
        "Addr": [127, 0, 0, 1],
        "Port": 5000,
        "Tags": {
            "role": "test"
        },
        "Status": "alive",
        "ProtocolMin": 0,
        "ProtocolMax": 3,
        "ProtocolCur": 2,
        "DelegateMin": 0,
        "DelegateMax": 1,
        "DelegateCur": 1,
        },
        ...]
    }
```

### members-wan

The members-wan command is used to return all the known wan members and associated
information. Only agents in server mode will respond to this command.

There is no request body, and the response is the same as `members-lan`

### monitor

The monitor command subscribes the channel to log messages from the Agent.

The request is like:

```
    {"LogLevel": "DEBUG"}
```

This subscribes the client to all messages of at least DEBUG level.

The server will respond with a standard response header indicating if the monitor
was successful. However, now as logs occur they will be sent and tagged with
the same `Seq` as the monitor command that matches.

Assume we issued the previous monitor command with Seq `50`,
we may start getting messages like:

```
    {"Seq": 50, "Error": ""}
    {"Log": "2013/12/03 13:06:53 [INFO] agent: Received event: member-join"}
```

It is important to realize that these messages are sent asyncronously,
and not in response to any command. That means if a client is streaming
commands, there may be logs streamed while a client is waiting for a
response to a command. This is why the `Seq` must be used to pair requests
with their corresponding responses.

The client can only be subscribed to at most a single monitor instance.
To stop streaming, the `stop` command is used.

### stop

The stop command is used to stop a monitor.
The request looks like:

```
    {"Stop": 50}
```

This unsubscribes the client from the monitor with `Seq` value of 50.

There is no special response body.

### leave

The leave command is used trigger a graceful leave and shutdown.
There is no request body, or special response body.

### stats

The stats command is used to provide operator information for debugginer.
There is no request body, the response body looks like:

```
    {
        "agent": {
            "check_monitors": 0,
            ...
        },
        "consul: {
            "server": "true",
            ...
        },
        ...
    }
```

