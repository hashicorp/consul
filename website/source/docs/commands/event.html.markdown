---
layout: "docs"
page_title: "Commands: Event"
sidebar_current: "docs-commands-event"
---

# Serf Event

Command: `serf event`

The `serf event` command dispatches a custom user event into a Serf cluster,
leveraging Serf's gossip layer for scalable broadcasting of the event to
clusters of any size.

Nodes in the cluster can listen for these custom events and react to them.
Example use cases of custom events are to trigger deploys across web nodes
by sending a "deploy" event, possibly with a commit payload. Another use
case might be to send a "restart" event, asking the cluster members to
restart.

Ultimately, `serf event` is used to send custom events of your choosing
that you can respond to in _any way_ you want. The power in Serf's custom
events is the scalability over other systems.

## Usage

Usage: `serf event [options] name [payload]`

The following command-line options are available for this command.
Every option is optional:

* `-coalesce=true/false` - Sets whether or not this event can be coalesced
  by Serf. By default this is set to true. Read the section on event
  coalescing for more information on what this means.

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:7373" which is the default RPC address of a Serf agent.

## Sending an Event

To send an event, use `serf event NAME` where NAME is the name of the
event to send. This call will return immediately, and Serf will use its
gossip layer to broadcast the event.

An event may also contain a payload. You may specify the payload using
the second parameter. For example: `serf event deploy 1234567890` would
send the "deploy" event with "1234567890" as the payload.

## Receiving an Event

The events can be handled by registering an
[event handler](/docs/agent/event-handlers.html) with the Serf agent. The
documentation for how the user event is dispatched is all contained within
that linked page.

## Event Coalescing

By default, Serf coalesces events of the same name within a short time
period. This means that if many events of the same name are received within
a short amount of time, the event handler is only invoked once, with the
last event of that name received during that time period.

Event coalescense works great for idempotent events such as "restart" or
events where only the last value in the payload really matters, like the
commit in a "deploy" event. In these cases, things just work.

Some events, however, shouldn't be coalesced. For example, if you had
an event "queue" that queues some item, then you want to make sure all
of those events are seen, even if many are sent in a short period of time.
In this case, the `-coalesce=false` flag should be passed to the
`serf event` command.

If you send some events of the same name with coalescence enabled and some
without, then only the events that have coalescing enabled will actually
coalesce. The others will always be delivered.
