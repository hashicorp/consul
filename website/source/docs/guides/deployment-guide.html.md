---
layout: "docs"
page_title: "Consul Deployment Guide"
sidebar_current: "docs-guides-deployment-guide"
description: |-
  This deployment guide covers the steps required to install and
  configure a single HashiCorp Consul cluster as defined in the
  Consul Reference Architecture.
ea_version: 1.4
---

# Consul Deployment Guide

This deployment guide covers the steps required to install and configure a single HashiCorp Consul cluster as defined in the [Consul Reference Architecture](/docs/guides/deployment.html).

These instructions are for installing and configuring Consul on Linux hosts running the systemd system and service manager.

## Reference Material

This deployment guide is designed to work in combination with the [Consul Reference Architecture](/docs/guides/deployment.html). Although not a strict requirement to follow the Consul Reference Architecture, please ensure you are familiar with the overall architecture design; for example installing Consul server agents on multiple physical or virtual (with correct anti-affinity) hosts for high-availability.

## Overview

To provide a highly-available single cluster architecture, we recommend Consul server agents be deployed to more than one host, as shown in the [Consul Reference Architecture](/docs/guides/deployment.html).

![Reference Diagram](/assets/images/consul-arch-single.png "Reference Diagram")

These setup steps should be completed on all Consul hosts.

- [Download Consul](#download-consul)
- [Install Consul](#install-consul)
- [Configure systemd](#configure-systemd)
- Configure Consul [(server)](#configure-consul-server-) or [(client)](#configure-consul-client-)
- [Start Consul](#start-consul)

## Download Consul

Precompiled Consul binaries are available for download at [https://releases.hashicorp.com/consul/](https://releases.hashicorp.com/consul/) and Consul Enterprise binaries are available for download by following the instructions made available to HashiCorp Consul customers.

You should perform checksum verification of the zip packages using the SHA256SUMS and SHA256SUMS.sig files available for the specific release version. HashiCorp provides [a guide on checksum verification](https://www.hashicorp.com/security.html) for precompiled binaries.

```text
CONSUL_VERSION="x.x.x"
curl --silent --remote-name https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_linux_amd64.zip
curl --silent --remote-name https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_SHA256SUMS
curl --silent --remote-name https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_SHA256SUMS.sig
```

## Install Consul

Unzip the downloaded package and move the `consul` binary to `/usr/local/bin/`. Check `consul` is available on the system path.

```text
unzip consul_${CONSUL_VERSION}_linux_amd64.zip
sudo chown root:root consul
sudo mv consul /usr/local/bin/
consul --version
```

The `consul` command features opt-in autocompletion for flags, subcommands, and arguments (where supported). Enable autocompletion.

```text
consul -autocomplete-install
complete -C /usr/local/bin/consul consul
```

Create a unique, non-privileged system user to run Consul and create its data directory.

```text
sudo useradd --system --home /etc/consul.d --shell /bin/false consul
sudo mkdir --parents /opt/consul
sudo chown --recursive consul:consul /opt/consul
```

## Configure systemd

Systemd uses [documented sane defaults](https://www.freedesktop.org/software/systemd/man/systemd.directives.html) so only non-default values must be set in the configuration file.

Create a Consul service file at /etc/systemd/system/consul.service.

```text
sudo touch /etc/systemd/system/consul.service
```

Add this configuration to the Consul service file:

```text
[Unit]
Description="HashiCorp Consul - A service mesh solution"
Documentation=https://www.consul.io/
Requires=network-online.target
After=network-online.target
ConditionFileNotEmpty=/etc/consul.d/consul.hcl

[Service]
Type=notify
User=consul
Group=consul
ExecStart=/usr/local/bin/consul agent -config-dir=/etc/consul.d/
ExecReload=/usr/local/bin/consul reload
KillMode=process
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

The following parameters are set for the `[Unit]` stanza:

- [`Description`](https://www.freedesktop.org/software/systemd/man/systemd.unit.html#Description=) - Free-form string describing the consul service
- [`Documentation`](https://www.freedesktop.org/software/systemd/man/systemd.unit.html#Documentation=) - Link to the consul documentation
- [`Requires`](https://www.freedesktop.org/software/systemd/man/systemd.unit.html#Requires=) - Configure a requirement dependency on the network service
- [`After`](https://www.freedesktop.org/software/systemd/man/systemd.unit.html#Before=) - Configure an ordering dependency on the network service being started before the consul service
- [`ConditionFileNotEmpty`](https://www.freedesktop.org/software/systemd/man/systemd.unit.html#ConditionArchitecture=) - Check for a non-zero sized configuration file before consul is started

The following parameters are set for the `[Service]` stanza:

- [`User`, `Group`](https://www.freedesktop.org/software/systemd/man/systemd.exec.html#User=) - Run consul as the consul user
- [`ExecStart`](https://www.freedesktop.org/software/systemd/man/systemd.service.html#ExecStart=) - Start consul with the `agent` argument and path to the configuration file
- [`ExecReload`](https://www.freedesktop.org/software/systemd/man/systemd.service.html#ExecReload=) - Send consul a reload signal to trigger a configuration reload in consul
- [`KillMode`](https://www.freedesktop.org/software/systemd/man/systemd.kill.html#KillMode=) - Treat consul as a single process
- [`Restart`](https://www.freedesktop.org/software/systemd/man/systemd.service.html#RestartSec=) - Restart consul unless it returned a clean exit code
- [`LimitNOFILE`](https://www.freedesktop.org/software/systemd/man/systemd.exec.html#Process%20Properties) - Set an increased Limit for File Descriptors

The following parameters are set for the `[Install]` stanza:

- [`WantedBy`](https://www.freedesktop.org/software/systemd/man/systemd.unit.html#WantedBy=) - Creates a weak dependency on consul being started by the multi-user run level

## Configure Consul (server)

Consul uses [documented sane defaults](/docs/agent/options.html) so only non-default values must be set in the configuration file. Configuration can be read from multiple files and is loaded in lexical order. See the [full description](/docs/agent/options.html) for more information about configuration loading and merge semantics.

Consul server agents typically require a superset of configuration required by Consul client agents. We will specify common configuration used by all Consul agents in `consul.hcl` and server specific configuration in `server.hcl`.

### General configuration

Create a configuration file at `/etc/consul.d/consul.hcl`:

```text
sudo mkdir --parents /etc/consul.d
sudo touch /etc/consul.d/consul.hcl
sudo chown --recursive consul:consul /etc/consul.d
sudo chmod 640 /etc/consul.d/consul.hcl
```

Add this configuration to the `consul.hcl` configuration file:

~> **NOTE** Replace the `datacenter` parameter value with the identifier you will use for the datacenter this Consul cluster is deployed in. Replace the `encrypt` parameter value with the output from running `consul keygen` on any host with the `consul` binary installed.

```hcl
datacenter = "dc1"
data_dir = "/opt/consul"
encrypt = "Luj2FZWwlt8475wD1WtwUQ=="
```

- [`datacenter`](/docs/agent/options.html#_datacenter) - The datacenter in which the agent is running.
- [`data_dir`](/docs/agent/options.html#_data_dir) - The data directory for the agent to store state.
- [`encrypt`](/docs/agent/options.html#_encrypt) - Specifies the secret key to use for encryption of Consul network traffic.

### ACL configuration

The [ACL](/docs/guides/acl.html) guide provides instructions on configuring and enabling ACLs.

### Cluster auto-join

The `retry_join` parameter allows you to configure all Consul agents to automatically form a cluster using a common Consul server accessed via DNS address, IP address or using Cloud Auto-join. This removes the need to manually join the Consul cluster nodes together.

Add the retry_join parameter to the `consul.hcl` configuration file:

~> **NOTE** Replace the `retry_join` parameter value with the correct DNS address, IP address or [cloud auto-join configuration](/docs/agent/cloud-auto-join.html) for your environment.

```hcl
retry_join = ["172.16.0.11"]
```

- [`retry_join`](/docs/agent/options.html#retry-join) - Address of another agent to join upon starting up.

### Performance stanza

The [`performance`](/docs/agent/options.html#performance) stanza allows tuning the performance of different subsystems in Consul.

Add the performance configuration to the `consul.hcl` configuration file:

```hcl
performance {
  raft_multiplier = 1
}
```

- [`raft_multiplier`](/docs/agent/options.html#raft_multiplier) - An integer multiplier used by Consul servers to scale key Raft timing parameters. Setting this to a value of 1 will configure Raft to its highest-performance mode, equivalent to the default timing of Consul prior to 0.7, and is recommended for production Consul servers.

For more information on Raft tuning and the `raft_multiplier` setting, see the [server performance](/docs/install/performance.html) documentation.

### Telemetry stanza

The [`telemetry`](/docs/agent/options.html#telemetry) stanza specifies various configurations for Consul to publish metrics to upstream systems.

If you decide to configure Consul to publish telemtery data, you should review the [telemetry configuration section](/docs/agent/options.html#telemetry) of our documentation.

### TLS configuration

The [Creating Certificates](/docs/guides/creating-certificates.html) guide provides instructions on configuring and enabling TLS.

### Server configuration

Create a configuration file at `/etc/consul.d/server.hcl`:

```text
sudo mkdir --parents /etc/consul.d
sudo touch /etc/consul.d/server.hcl
sudo chown --recursive consul:consul /etc/consul.d
sudo chmod 640 /etc/consul.d/server.hcl
```

Add this configuration to the `server.hcl` configuration file:

~> **NOTE** Replace the `bootstrap_expect` value with the number of Consul servers you will use; three or five [is recommended](/docs/internals/consensus.html#deployment-table).

```hcl
server = true
bootstrap_expect = 3
```

- [`server`](/docs/agent/options.html#_server) -  This flag is used to control if an agent is in server or client mode.
- [`bootstrap-expect`](/docs/agent/options.html#_bootstrap_expect) - This flag provides the number of expected servers in the datacenter. Either this value should not be provided or the value must agree with other servers in the cluster.

### Consul UI

Consul features a web-based user interface, allowing you to easily view all services, nodes, intentions and more using a graphical user interface, rather than the CLI or API.

~> **NOTE** You should consider running the Consul UI on select Consul hosts rather than all hosts.

Optionally, add the UI configuration to the `server.hcl` configuration file to enable the Consul UI:

```hcl
ui = true
```

## Configure Consul (client)

Consul client agents typically require a subset of configuration required by Consul server agents. All Consul clients can use the `consul.hcl` file created when [configuring the Consul servers](#general-configuration). If you have added host-specific configuration such as identifiers, you will need to set these individually.

## Start Consul

Enable and start Consul using the systemctl command responsible for controlling systemd managed services. Check the status of the consul service using systemctl.

```text
sudo systemctl enable consul
sudo systemctl start consul
sudo systemctl status consul
```

## Backups

Creating server backups is an important step in production deployments. Backups provide a mechanism for the server to recover from an outage (network loss, operator error, or a corrupted data directory). All agents write to the `-data-dir` before commit. This directory persists the local agent’s state and &mdash; in the case of servers &mdash; it also holds the Raft information.

Consul provides the [snapshot](/docs/commands/snapshot.html) command which can be run using the CLI command or the API. The `snapshot` command saves the point-in-time snapshot of the state of the Consul servers which includes KV entries, the service catalog, prepared queries, sessions, and ACL.

With [Consul Enterprise](/docs/commands/snapshot/agent.html), the `snapshot agent` command runs periodically and writes to local or remote storage (such as Amazon S3).

By default, all snapshots are taken using `consistent` mode where requests are forwarded to the leader which verifies that it is still in power before taking the snapshot. Snapshots will not be saved if the clusted is degraded or if no leader is available. To reduce the burden on the leader, it is possible to [run the snapshot](/docs/commands/snapshot/save.html) on any non-leader server using `stale` consistency mode:

```text
consul snapshot save -stale backup.snap
```

This spreads the load across nodes at the possible expense of losing full consistency guarantees. Typically this means that a very small number of recent writes may not be included. The omitted writes are typically limited to data written in the last `100ms` or less from the recovery point. This is usually suitable for disaster recovery. However, the system can’t guarantee how stale this may be if executed against a partitioned server.

## Next Steps

- Read [Monitoring Consul with Telegraf](/docs/guides/monitoring-telegraf.html)
  for an example guide to monitoring Consul for improved operational visibility.

- Read [Outage Recovery](/docs/guides/outage.html) to learn the steps required
  for recovery from a Consul outage due to a majority of server nodes in a
  datacenter being lost.

- Read [Server Performance](/docs/install/performance.html) to learn about
  additional configuration that benefits production deployments.
