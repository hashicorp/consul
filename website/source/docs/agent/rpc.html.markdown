---
layout: "docs"
page_title: "RPC"
sidebar_current: "docs-agent-rpc"
---

# RPC Protocol

The Serf agent provides a complete RPC mechanism that can
be used to control the agent programmatically. This RPC
mechanism is the same one used by the CLI, but can be
used by other applications to easily leverage the power
of Serf without directly embedding. Additionally, it can
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
* event - Fires a new user event
* force-leave - Removes a failed node from the cluster
* join - Requests Serf join another node
* members - Returns the list of members
* stream - Starts streaming events over the connection
* monitor - Starts streaming logs over the connection
* stop - Stops streaming logs or events
* leave - Serf agent performs a graceful leave and shutdown

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

### event

The event command is used to fire a new user event. It takes the
following request body:

```
	{"Name": "foo", "Payload": "test payload", "Coalesce": true}
```

The `Name` is a string, but `Payload` is just opaque bytes. Coalesce
is used to control if Serf should enable [event coalescing](/docs/commands/event.html).

There is no special response body.

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
    {"Existing": ["192.168.0.1:6000", "192.168.0.2:6000"], "Replay": false}
```

The `Existing` nodes are each contacted, and `Replay` controls if we will replay
old user events or if they will simply be ignored. The response body in addition
to the header is returned. The body looks like:

```
    {"Num": 2}
```

The body returns the number of nodes successfully joined.

### members

The members command is used to return all the known members and associated
information. There is no request body, but the response looks like:

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

### stream

The stream command is used to subscribe to a stream of all events
matching a given type filter. Events will continue to be sent until
the stream is stopped. The request body looks like:

```
    {"Type": "member-join,user:deploy"}`
```

The format of type is the same as the [event handler](/docs/agent/event-handlers.html),
except no script is specified. The one exception is that `"*"` can be specified to
subscribe to all events.

The server will respond with a standard response header indicating if the stream
was successful. However, now as events occur they will be sent and tagged with
the same `Seq` as the stream command that matches.

Assume we issued the previous stream command with Seq `50`,
we may start getting messages like:

```
    {"Seq": 50, "Error": ""}
    {
        "Event": "user",
        "LTime": 123,
        "Name": "deploy",
        "Payload": "9c45b87",
        "Coalesce": true,
    }

    {"Seq": 50, "Error": ""}
    {
        "Event": "member-join",
        "Members": [
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
            ...
        ]
    }
```

It is important to realize that these messages are sent asyncronously,
and not in response to any command. That means if a client is streaming
commands, there may be events streamed while a client is waiting for a
response to a command. This is why the `Seq` must be used to pair requests
with their corresponding responses.

There is no limit to the number of concurrent streams a client can request,
however a message is not deduplicated, so if multiple streams match a given
event, it will be sent multiple times with the corresponding `Seq` number.

To stop streaming, the `stop` command is used.

### monitor

The monitor command is similar to the stream command, but instead of
events it subscribes the channel to log messages from the Agent.

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

The stop command is used to stop either a stream or monitor.
The request looks like:

```
    {"Stop": 50}
```

This unsubscribes the client from the monitor and/or stream registered
with `Seq` value of 50.

There is no special response body.

### leave

The leave command is used trigger a graceful leave and shutdown.
There is no request body, or special response body.

