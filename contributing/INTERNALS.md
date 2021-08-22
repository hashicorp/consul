# Consul Internals Guide

This guide is intended to help folks who want to understand more about how Consul works from a code perspective, or who are thinking about contributing to Consul. For a high level overview of Consul's design, please see the [Consul Architecture Guide](https://www.consul.io/docs/internals/architecture.html) as a starting point.

## Architecture Overview

Consul is designed around the concept of a [Consul Agent](https://www.consul.io/docs/agent/basics.html). The agent is deployed as a single Go binary and runs on every node in a cluster.

A small subset of agents, usually 3 to 7, run in server mode and participate in the [Raft Consensus Protocol](https://www.consul.io/docs/internals/consensus.html). The Consul servers hold a consistent view of the state of the cluster, including the [service catalog](https://www.consul.io/api/catalog.html) and the [health state of services and nodes](https://www.consul.io/api/health.html) as well as other items like Consul's [key/value store contents](https://www.consul.io/api/kv.html). An agent in server mode is a superset of the client capabilities that follow.

All the remaining agents in a cluster run in client mode. Applications on client nodes use their local agent in client mode to [register services](https://www.consul.io/api/agent.html) and to discover other services or interact with the key/value store. For the latter queries, the agent sends RPC requests internally to one of the Consul servers for the information. None of the key/value data is on any of the client agents, for example, it's always fetched on the fly from a Consul server.

Both client and server mode agents participate in a [Gossip Protocol](https://www.consul.io/docs/internals/gossip.html) which provides two important mechanisms. First, it allows for agents to learn about all the other agents in the cluster, just by joining initially with a single existing member of the cluster. This allows clients to discover new Consul servers. Second, the gossip protocol provides a distributed failure detector, whereby the agents in the cluster randomly probe each other at regular intervals. Because of this failure detector, Consul can run health checks locally on each agent and just sent edge-triggered updates when the state of a health check changes, confident that if the agent dies altogether then the cluster will detect that. This makes Consul's health checking design very scaleable compared to centralized systems with a central polling type of design.

There are many other aspects of Consul that are well-covered in Consul's [Internals Guides](https://www.consul.io/docs/internals/index.html).

## Source Code Layout

### Shared Components

The components in this section are shared between Consul agents in client and server modes.

| Directory | Contents |
| --------- | -------- |
| [command/agent](https://github.com/hashicorp/consul/tree/main/command/agent) | This contains the actual CLI command implementation for the `consul agent` command, which mostly just invokes an agent object from the `agent` package. |
| [agent](https://github.com/hashicorp/consul/tree/main/agent) | This is where the agent object is defined, and the top level `agent` package has all of the functionality that's common to both client and server agents. This includes things like service registration, the HTTP and DNS endpoints, watches, and top-level glue for health checks. |
| [agent/config](https://github.com/hashicorp/consul/tree/main/agent/config) | This has all the user-facing [configuration](https://www.consul.io/docs/agent/options.html) processing code, as well as the internal configuration structure that's used by the agent. |
| [agent/checks](https://github.com/hashicorp/consul/tree/main/agent/checks) | This has implementations for the different [health check types](https://www.consul.io/docs/agent/checks.html). |
| [agent/ae](https://github.com/hashicorp/consul/tree/main/agent/ae), [agent/local](https://github.com/hashicorp/consul/tree/main/agent/local) | These are used together to power the agent's [Anti-Entropy Sync Back](https://www.consul.io/docs/internals/anti-entropy.html) process to the Consul servers. |
| [agent/router](https://github.com/hashicorp/consul/tree/main/agent/router), [agent/pool](https://github.com/hashicorp/consul/tree/main/agent/pool) | These are used for routing RPC queries to Consul servers and for connection pooling. |
| [agent/structs](https://github.com/hashicorp/consul/tree/main/agent/structs) | This has definitions of all the internal RPC protocol request and response structures. |

### Other Components

There are several other top-level packages used internally by Consul as well as externally by other applications.

| Directory | Contents |
| --------- | -------- |
| [api](https://github.com/hashicorp/consul/tree/main/api) | This `api` package provides an official Go API client for Consul, which is also used by Consul's [CLI](https://www.consul.io/docs/commands/index.html) commands to communicate with the local Consul agent. |
| [api/watch](https://github.com/hashicorp/consul/tree/main/api/watch) | This has implementation details for Consul's [watches](https://www.consul.io/docs/agent/watches.html), used both internally to Consul and by the [watch CLI command](https://www.consul.io/docs/commands/watch.html). |
| [website](https://github.com/hashicorp/consul/tree/main/website) | This has the full source code for [consul.io](https://www.consul.io/). Pull requests can update the source code and Consul's documentation all together. |
