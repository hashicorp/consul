---
layout: "docs"
page_title: "Using Consul With Containers"
sidebar_current: "docs-guides-consul-on-containers"
description: |-
  This guide describes how to run consul on containers, with docker as the primary focus. It also describes best practices when running a consul cluster in production on docker.
---

## Consul with Containers
This guide describes critical aspects of operating a consul cluster that's run inside containers. It primarily focuses on Docker. 

## Consul Official Docker Image

Consul's official docker image is available via `docker pull consul`. This will download a docker image that runs consul's latest stable released version.

More instructions on how to get started using this image are available at the [official docker repository page](https://hub.docker.com/_/consul/)

## Data Directory Persistence

The container exposes its data directory, `/consul/data` as a [volume](https://docs.docker.com/engine/tutorials/dockervolumes/) , which is a path where Consul will place its persisted state.

For client agents, this stores some information about the cluster and the client's health checks in case the container is restarted. If the volume on a client agent disappears, it doesn't affect cluster operations.

For server agents, this stores the client information plus snapshots and data related to the consensus algorithm and other state like Consul's key/value store and catalog. **Servers need the volume's data around when restarting containers to recover from outage scenarios.** Therefore, care must be taken by operators to make sure that volumes containing consul cluster data are not destroyed during container restarts, 

**We also recommend taking additional backups via [`consul snapshot`](https://www.consul.io/docs/commands/snapshot.html), and storing them in an external storage system like S3.**

## Networking
When run inside a container, Consul's IP addresses need to be setup properly. Consul has to be configured with an appropriate _cluster address_ as well as a _client address._ 

 * **Cluster Address** -  The address at which other Consul agents may contact a given agent. This is also referred to as the bind address. 

* ** Client Address** -  The address where other processes on the host contact Consul in order to make HTTP or DNS requests. 

You will need to tell Consul what its cluster address is when starting so that it binds to the correct interface and advertises a workable interface to the rest of the Consul agents.  There are two ways of doing this. 

1. _Environmental Variables_: Use the `CONSUL_CLIENT_INTERFACE` and `CONSUL_BIND_INTERFACE` environment variables. In the following example `eth0` is the network interface of the container:
```
docker run -d -e CONSUL_CLIENT_INTERFACE=eth0 -e CONSUL_BIND_INTERFACE='eth0' consul agent -server -bootstrap-expect=3
```

2. _Address templates_: You can declaratively specify the client and cluster addresses using the formats described in the [go-socketaddr](https://github.com/hashicorp/go-sockaddr) library. In the following example, the client and bind addresses are declaratively specified for the container network interface 'eth0'
```
docker run consul agent -server -client='{{ GetInterfaceIP "eth0" }}' -bind='{{ GetInterfaceIP "eth0" }}' -bootstrap-expect=3
```

## Stopping and Restarting Containers
Consul containers can be stopped using the `docker stop <container_id>` command and restarted using `docker start <container_id>`. As long as there are enough servers in the cluster to maintain [quorum](https://www.consul.io/docs/internals/consensus.html#deployment-table), Consul's [Autopilot](https://www.consul.io/docs/guides/autopilot.html) feature will handle removing servers whose containers were stopped. Make sure that [Autopilot settings](https://www.consul.io/docs/agent/options.html#autopilot) are appropriate. 

* `cleanup_dead_servers` must be set to true to make sure that a stopped container is removed from the cluster. 
* `last_contact_threshold` should be reasonably small, so that dead servers are removed quickly. 
* `server_stabilization_time`should be sufficiently large(in the order of several seconds) so that unstable servers are not added to the cluster until they stabilize. 

If a container that was running a leader is stopped, leader election will be triggered causing another server in the cluster to assume leadership. 

When a previously stopped server container is restarted using `docker start <container_id>`,  and it is configured to obtain a new IP, Autopilot will add it back to the set of Raft peers with the same node-id and the new IP address, after which it can participate as a server again. 

## Known Issues
**Consul does not currently gracefully handle the situation where all nodes in the cluster running inside a container are restarted at the same time, and they all obtain new IP addresses.** This is because the underlying Raft layer persists the IP address and needs it for leader election operations. Operators must carefully orchestrate restarts of consul containers that have ephemeral IP addresses to do restarts in small numbers, so that they can gracefully leave the cluster and re-join with their new IP address. 



