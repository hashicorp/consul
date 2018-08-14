---
layout: "docs"
page_title: "Using Consul with Containers"
sidebar_current: "docs-guides-consul-containers"
description: |-
  This guide describes how to run Consul on containers, with Docker as the primary focus. It also describes best practices when running a Consul cluster in production on Docker.
---

## Consul with Containers
This guide describes critical aspects of operating a Consul cluster that's run inside containers. It primarily focuses on the Docker container runtime, but the principles largely apply to rkt, oci, and other container runtimes as well.

## Consul Official Docker Image
Consul's official Docker images are tagged with version numbers. For example, `docker pull consul:0.9.0` will pull the 0.9.0 Consul release image.

For major releases, make sure to read our [upgrade guides](/docs/upgrade-specific.html) before upgrading a cluster.

To get a development mode Consul instance running the latest version, run `docker run consul`.

More instructions on how to get started using this image are available at the [official Docker repository page](https://store.docker.com/images/consul)

## Data Directory Persistence

The container exposes its data directory, `/consul/data`, as a [volume](https://docs.docker.com/engine/tutorials/dockervolumes/). This is where Consul will store its persisted state.

For clients, this stores some information about the cluster and the client's services and health checks in case the container is restarted. If the volume on a client disappears, it doesn't affect cluster operations.

For servers, this stores the client information plus snapshots and data related to the consensus algorithm and other state like Consul's key/value store and catalog. **Servers need the volume's data to be available when restarting containers to recover from outage scenarios.** Therefore, care must be taken by operators to make sure that volumes containing consul cluster data are not destroyed during container restarts.

~> We also recommend taking additional backups via [`consul snapshot`](/docs/commands/snapshot.html), and storing them externally.

## Configuration
The container has a Consul configuration directory set up at `/consul/config` and the agent will load any configuration files placed here by binding a volume or by composing a new image and adding files.

Note that the configuration directory is not exposed as a volume, and will not persist. Consul uses it only during start up and does not store any state there.

Configuration can also be added by passing the configuration JSON via environment variable CONSUL_LOCAL_CONFIG. Example:

```sh
 $ docker run \
   -d \
   -e CONSUL_LOCAL_CONFIG='{
    "datacenter":"us_west",
    "server":true,
    "enable_debug":true
    }' \
   consul agent -server -bootstrap-expect=3
```

## Networking
When running inside a container, Consul must be configured with an appropriate _cluster address_ and _client address_. In some cases, it may also require configuring an _advertise address_.

 * **Cluster Address** -  The address at which other Consul agents may contact a given agent. This is also referred to as the bind address.

 * **Client Address** -  The address where other processes on the host contact Consul in order to make HTTP or DNS requests. Consider setting this to localhost or `127.0.0.1` to only allow processes on the same container to make HTTP/DNS requests.

 * **Advertise Address** - The advertise address is used to change the address that we advertise to other nodes in the cluster. This defaults to the bind address. Consider using this if you use NAT in your environment, or in scenarios where you have a routable address that cannot be bound.

You will need to tell Consul what its cluster address is when starting so that it binds to the correct interface and advertises a workable interface to the rest of the Consul agents.  There are two ways of doing this:

1. Environment Variables: Use the `CONSUL_CLIENT_INTERFACE` and `CONSUL_BIND_INTERFACE` environment variables. In the following example `eth0` is the network interface of the container.

    ```sh
    $ docker run \
      -d \
      -e CONSUL_CLIENT_INTERFACE='eth0' \
      -e CONSUL_BIND_INTERFACE='eth0' \
      consul agent -server -bootstrap-expect=3
    ```
2. Address Templates: You can declaratively specify the client and cluster addresses using the formats described in the [go-socketaddr](https://github.com/hashicorp/go-sockaddr) library.
In the following example, the client and bind addresses are declaratively specified for the container network interface 'eth0'

    ```sh
    $ docker run \
      consul agent -server \
      -client='{{ GetInterfaceIP "eth0" }}' \
      -bind='{{ GetInterfaceIP "eth0" }}' \
      -bootstrap-expect=3
    ```

## Stopping and Restarting Containers
The official Consul container supports stopping, starting, and restarting. To stop a container, run `docker stop`:

```sh
$ docker stop <container_id>
```

To start a container, run `docker start`:

```sh
$ docker start <container_id>
```

To do an in-memory reload, send a SIGHUP to the container:

```sh
$ docker kill --signal=HUP <container_id>
```

As long as there are enough servers in the cluster to maintain [quorum](/docs/internals/consensus.html#deployment-table), Consul's [Autopilot](/docs/guides/autopilot.html) feature will handle removing servers whose containers were stopped. Autopilot's default settings are already configured correctly. If you override them, make sure that the following [settings](/docs/agent/options.html#autopilot) are appropriate.

* `cleanup_dead_servers` must be set to true to make sure that a stopped container is removed from the cluster.
* `last_contact_threshold` should be reasonably small, so that dead servers are removed quickly.
* `server_stabilization_time` should be sufficiently large (on the order of several seconds) so that unstable servers are not added to the cluster until they stabilize.

If the container running the currently-elected Consul server leader is stopped, a leader election will trigger. This event will cause a new Consul server in the cluster to assume leadership.

When a previously stopped server container is restarted using `docker start <container_id>`,  and it is configured to obtain a new IP, Autopilot will add it back to the set of Raft peers with the same node-id and the new IP address, after which it can participate as a server again.

## Known Issues
**All nodes changing IP addresses** Prior to Consul 0.9.3, Consul did not gracefully handle the situation where all nodes in the cluster running inside a container are restarted at the same time, and they all obtain new IP addresses. This has been [fixed](https://github.com/hashicorp/consul/issues/1580) since Consul 0.9.3, and requires `"raft_protocol"` to be set to `"3"` in the configs in Consul 0.9.3. Consul 1.0 makes raft protocol 3 the default.

**Snapshot close error** Due to a [known issue](https://github.com/docker/libnetwork/issues/1204) with half close support in Docker, you will see an error message `[ERR] consul: Failed to close snapshot: write tcp <source>-><destination>: write: broken pipe` when saving snapshots. This does not affect saving and restoring snapshots when running in Docker.
