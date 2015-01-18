---
layout: "docs"
page_title: "Status (HTTP)"
sidebar_current: "docs-agent-http-status"
description: >
  The Status endpoints are used to get information about the status
  of the Consul cluster.
---

# Status HTTP Endpoint

The Status endpoints are used to get information about the status
of the Consul cluster. These are generally very low level, and not really
useful for clients.

The following endpoints are supported:

* [`/v1/status/leader`](#status_leader) : Returns the current Raft leader
* [`/v1/status/peers`](#status_peers) : Returns the current Raft peer set

### <a name="status_leader"></a> /v1/status/leader

This endpoint is used to get the Raft leader for the datacenter
the agent is running in. It returns only an address like:

```text
"10.1.10.12:8300"
```

### <a name="status_peers"></a> /v1/status/peers

This endpoint is used to get the Raft peers for the datacenter
the agent is running in. It returns a list of addresses like:

```javascript
[
  "10.1.10.12:8300",
  "10.1.10.11:8300",
  "10.1.10.10:8300"
]
```
