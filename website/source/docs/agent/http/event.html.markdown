---
layout: "docs"
page_title: "Events (HTTP)"
sidebar_current: "docs-agent-http-event"
description:  >
  The Event endpoints are used to fire new events and to query the
  available events
---

# Event HTTP Endpoint

The Event endpoints are used to fire new events and to query the available
events.

The following endpoints are supported:

* [`/v1/event/fire/<name>`](#event_fire): Fires a new user event
* [`/v1/event/list`](#event_list): Lists the most recent events an agent has seen.

### <a name="event_fire"></a> /v1/event/fire/\<name\>

The fire endpoint is used to trigger a new user event. A user event
needs a name, and optionally takes a number of parameters.

By default, the agent's local datacenter is used, but another datacenter
can be specified using the "?dc=" query parameter.

The fire endpoint expects a PUT request, with an optional body.
The body contents are opaque to Consul, and become the "payload"
of the event. Any names starting with the "_" prefix should be considered
reserved, and for Consul's internal use.

The `?node=`, `?service=`, and `?tag=` query parameters may optionally
be provided. They respectively provide a regular expression to filter
by node name, service, and service tags.

The return code is 200 on success, along with a body like:

```javascript
{
  "ID": "b54fe110-7af5-cafc-d1fb-afc8ba432b1c",
  "Name": "deploy",
  "Payload": null,
  "NodeFilter": "",
  "ServiceFilter": "",
  "TagFilter": "",
  "Version": 1,
  "LTime": 0
}
```

This is used to provide the ID of the newly fired event.

### <a name="event_list"></a> /v1/event/list

This endpoint is hit with a GET and returns the most recent
events known by the agent. As a consequence of how the
[event command](/docs/commands/event.html) works, each agent
may have a different view of the events. Events are broadcast using
the [gossip protocol](/docs/internals/gossip.html), which means
they have no total ordering, nor do they make a promise of delivery.

Additionally, each node applies the node, service and tag filters
locally before storing the event. This means the events at each agent
may be different depending on their configuration.

This endpoint does allow for filtering on events by name by providing
the `?name=` query parameter.

To support [watches](/docs/agent/watches.html), this endpoint supports
blocking queries. However, the semantics of this endpoint are slightly
different. Most blocking queries provide a monotonic index, and block
until a newer index is available. This can be supported as a consequence
of the total ordering of the [consensus protocol](/docs/internals/consensus.html).
With gossip, there is no ordering, and instead `X-Consul-Index` maps
to the newest event that matches the query.

In practice, this means the index is only useful when used against a
single agent, and has no meaning globally. Because Consul defines
the index as being opaque, clients should not be expecting a natural
ordering either.

Agents only buffer the most recent entries. The number of entries should
not be depended upon, but currently defaults to 256. This value could
change in the future. The buffer should be large enough for most clients
and watches.

It returns a JSON body like this:

```javascript
[
  {
    "ID": "b54fe110-7af5-cafc-d1fb-afc8ba432b1c",
    "Name": "deploy",
    "Payload": "MTYwOTAzMA==",
    "NodeFilter": "",
    "ServiceFilter": "",
    "TagFilter": "",
    "Version": 1,
    "LTime": 19
  },
  ...
]
```
