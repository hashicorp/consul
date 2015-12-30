---
layout: "docs"
page_title: "Coordinate (HTTP)"
sidebar_current: "docs-agent-http-coordinate"
description: >
  The Coordinate endpoint is used to query for the network coordinates for
  nodes in the local datacenter as well as Consul servers in the local
  datacenter and remote datacenters.
---

# Coordinate HTTP Endpoint

The Coordinate endpoint is used to query for the network coordinates for nodes
in the local datacenter as well as Consul servers in the local datacenter and
remote datacenters.

See the [Network Coordinates](/docs/internals/coordinates.html) internals guide
for more information on how these coordinates are computed, and for details on
how to perform calculations with them.

The following endpoints are supported:

* [`/v1/coordinate/datacenters`](#coordinate_datacenters) : Queries for WAN coordinates of Consul servers
* [`/v1/coordinate/nodes`](#coordinate_nodes) : Queries for LAN coordinates of Consul nodes

### <a name="coordinate_datacenters"></a> /v1/coordinate/datacenters

This endpoint is hit with a GET and returns the WAN network coordinates for
all Consul servers, organized by DCs.

It returns a JSON body like this:

```javascript
[
  {
    "Datacenter": "dc1",
    "Coordinates": [
      {
        "Node": "agent-one",
        "Coord": {
          "Adjustment": 0,
          "Error": 1.5,
          "Height": 0,
          "Vec": [0,0,0,0,0,0,0,0]
        }
      }
    ]
  }
]
```

This endpoint serves data out of the server's local Serf data about the WAN, so
its results may vary as requests are handled by different servers in the
cluster. Also, it does not support blocking queries or any consistency modes.

### <a name=""coordinate_nodes></a> /v1/coordinate/nodes

This endpoint is hit with a GET and returns the LAN network coordinates for
all nodes in a given DC. By default, the datacenter of the agent is queried;
however, the dc can be provided using the "?dc=" query parameter.

It returns a JSON body like this:

```javascript
[
  {
    "Node": "agent-one",
    "Coord": {
      "Adjustment": 0,
      "Error": 1.5,
      "Height": 0,
      "Vec": [0,0,0,0,0,0,0,0]
    }
  }
]
```

This endpoint supports blocking queries and all consistency modes.
