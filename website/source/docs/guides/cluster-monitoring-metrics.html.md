---
layout: "docs"
page_title: "Consul Cluster Monitoring & Metrics"
sidebar_current: "docs-guides-cluster-monitoring-metrics"
description: After setting up your first datacenter, it is an ideal time to make sure your cluster is healthy and establish a baseline.
---

# Consul Cluster Monitoring and Metrics

After setting up your first datacenter, it is an ideal time to make sure your cluster is healthy and establish a baseline. This guide will cover several types of metrics in two sections: Consul health and server health. 

**Consul health**:

- Transaction timing
- Leadership changes
- Autopilot
- Garbage collection

**Server health**:

- File descriptors
- CPU usage
- Network activity
- Disk activity
- Memory usage

For each type of metric, we will review their importance and help identify when a metric is indicating a healthy or unhealthy state. 

First, we need to understand the three methods for collecting metrics. We will briefly cover using SIGUSR1, the HTTP API, and telemetry. 

Before starting this guide, we recommend configuring [ACLs](/docs/guides/acl.html).

## How to Collect Metrics

There are three methods for collecting metrics. The first, and simplest, is to use `SIGUSR1` for a one-time dump of current telemetry values. The second method is to get a similar one-time dump using the HTTP API. The third method, and the one most commonly used for long-term monitoring, is to enable telemetry in the Consul configuration file. 

### SIGUSR1 for Local Use

To get a one-time dump of current metric values, we can send the `SIGUSR1` signal to the Consul process.

```sh
$ kill -USR1 <process_id>
```
This will send the output to the system logs, such as `/var/log/messages` or to `journald`. If you are monitoring the Consul process in the terminal via `consul monitor`, you will see the metrics in the output.

Although this is the easiest way to get a quick read of a single Consul agent’s health, it is much more useful to look at how the values change over time. 

### API GET Request

Next let’s use the HTTP API to quickly collect metrics with curl.

```ssh
$ curl http://127.0.0.1:8500/v1/agent/metrics
```

In production you will want to set up credentials with an ACL token and [enable TLS](/docs/agent/encryption.html) for secure communications. Once ACLs have been configured, you can pass a token with the request.

```sh
$ curl \
    --header "X-Consul-Token: <YOUR_ACL_TOKEN>" \
    https://127.0.0.1:8500/v1/agent/metrics
```

In addition to being a good way to quickly collect metrics, it can be added to a script or it can be used with monitoring agents that support HTTP scraping, such as Prometheus, to visualize the data.

### Enable Telemetry

Finally, Consul can be configured to send telemetry data to a remote monitoring system. This allows you to monitor the health of agents over time, spot trends, and plan for future needs. You will need a monitoring agent and console for this. 

Consul supports the following telemetry agents:
* Circonus 
* DataDog (via `dogstatsd`)
* StatsD (via `statsd`, `statsite`, `telegraf`, etc.)

If you are using StatsD, you will also need a compatible database and server, such as Grafana, Chronograf, or Prometheus.

Telemetry can be enabled in the agent configuration file, for example `server.hcl`. Telemetry can be enabled on any agent, client or server. Normally, you would at least enable it on all the servers (both voting and non-voting) to monitor the health of the entire cluster. 

An example snippet of `server.hcl` to send telemetry to DataDog looks like this:

```json
  "telemetry": {
    "dogstatsd_addr": "localhost:8125",
    "disable_hostname": true
  }
```

When enabling telemetry on an existing cluster, the Consul process will need to be reloaded. This can be done with `consul reload` or `kill -HUP <process_id>`. It is recommended to reload the servers one at a time, starting with the non-leaders. 

## Consul Health

The Consul health metrics reveal information about the Consul cluster. They include performance metrics for the key value store, transactions, raft, leadership changes, autopilot tuning, and garbage collection. 

### Transaction Timing

The following metrics indicate how long it takes to complete write operations
in various parts, including Consul KV and Raft from the Consul server. Generally, these values should remain reasonably consistent and no more than a few milliseconds each. 

| Metric Name              | Description |
| :----------------------- | :---------- |
| `consul.kvs.apply`       | Measures the time it takes to complete an update to the KV store. |
| `consul.txn.apply`       | Measures the time spent applying a transaction operation. |
| `consul.raft.apply`      | Counts the number of Raft transactions occurring over the interval. |
| `consul.raft.commitTime` | Measures the time it takes to commit a new entry to the Raft log on the leader. |

Sudden changes in any of the timing values could be due to unexpected load on the Consul servers or due to problems on the hosts themselves. Specifically, if any of these metrics deviate more than 50% from the baseline over the previous hour, this indicates an issue. Below are examples of healthy transaction metrics.

```sh
'consul.raft.apply': Count: 1 Sum: 1.000 LastUpdated: 2018-11-16 10:55:03.673805766 -0600 CST m=+97598.238246167
'consul.raft.commitTime': Count: 1 Sum: 0.017 LastUpdated: 2018-11-16 10:55:03.673840104 -0600 CST m=+97598.238280505
```

### Leadership Changes

In a healthy environment, your Consul cluster should have a stable leader. There shouldn’t be any leadership changes unless you manually change leadership (by taking a server out of the cluster, for example). If there are unexpected elections or leadership changes, you should investigate possible network issues between the Consul servers. Another possible cause could be that the Consul servers are unable to keep up with the transaction load. 

Note: These metrics are reported by the follower nodes, not by the leader.

| Metric Name | Description |
| :---------- | :---------- |
| `consul.raft.leader.lastContact` | Measures the time since the leader was last able to contact the follower nodes when checking its leader lease. |
| `consul.raft.state.candidate` | Increments when a Consul server starts an election process. |
| `consul.raft.state.leader` | Increments when a Consul server becomes a leader. |

If the `candidate` or `leader` metrics are greater than 0 or the `lastContact` metric is greater than 200ms, you should look into one of the possible causes described above. Below are examples of healthy leadership metrics. 

```sh
'consul.raft.leader.lastContact': Count: 4 Min: 10.000 Mean: 31.000 Max: 50.000 Stddev: 17.088 Sum: 124.000 LastUpdated: 2018-12-17 22:06:08.872973122 +0000 UTC m=+3553.639379498
'consul.raft.state.leader': Count: 1 Sum: 1.000 LastUpdated: 2018-12-17 22:05:49.104580236 +0000 UTC m=+3533.870986584
'consul.raft.state.candidate': Count: 1 Sum: 1.000 LastUpdated: 2018-12-17 22:05:49.097186444 +0000 UTC m=+3533.863592815
```

### Autopilot

The autopilot metric is a boolean. A value of 1 indicates a healthy cluster and 0 indicates an unhealthy state.

| Metric Name | Description |
| :---------- | :---------- |
| `consul.autopilot.healthy` | Tracks the overall health of the local server cluster. If all servers are considered healthy by autopilot, this will be set to 1. If any are unhealthy, this will be 0. |

An alert should be setup for a returned value of 0. Below is an example of a healthy cluster according to the autopilot metric.

```sh
[2018-12-17 13:03:40 -0500 EST][G] 'consul.autopilot.healthy': 1.000
```

### Garbage Collection

Garbage collection (GC) pauses are a "stop-the-world" event, all runtime threads are blocked until GC completes. In a healthy environment these pauses should only last a few nanoseconds. If memory usage is high, the Go runtime may start the GC process so frequently that it will slow down Consul. You might observe more frequent leader elections or longer write times.

| Metric Name | Description |
| :---------- | :---------- |
| `consul.runtime.total_gc_pause_ns` | Number of nanoseconds consumed by stop-the-world garbage collection (GC) pauses since Consul started. |

If the value return is more than 2 seconds/minute, you should start investigating the cause. If it exceeds 5 seconds per minute, you should consider the cluster to be in a critical state and start ensuring failure recovery procedures are up-to-date and start investigating. Below is an example of healthy GC pause.

```sh
'consul.runtime.total_gc_pause_ns': 136603664.000
```

Note, `total_gc_pause_ns` is a cumulative counter, so in order to calculate rates, such as GC/minute, you will need to apply a function such as [non_negative_difference](https://docs.influxdata.com/influxdb/v1.5/query_language/functions/#non-negative-difference).

## Server Health 

The server metrics provide information about the health of your cluster including file handles, CPU usage, network activity, disk activity, and memory usage. 

### File Descriptors

The majority of Consul operations require a file descriptor handle, including receiving a connection from another host, sending data between servers, and writing snapshots to disk. If Consul runs out of handles, it will stop accepting connections. 

| Metric Name | Description |
| :---------- | :---------- |
| `linux_sysctl_fs.file-nr` | Number of file handles being used across all processes on the host. |
| `linux_sysctl_fs.file-max` | Total number of available file handles. |

By default, process and kernel limits are conservative, you may want to increase the limits beyond the defaults. If  the `linux_sysctl_fs.file-nr` value exceeds 80% of `linux_sysctl_fs.file-max`, the file handles should be increased. Below is an example of a file handle metric.

```sh
linux_sysctl_fs, host=statsbox, file-nr=768i, file-max=96763i 
```

### CPU Usage

Consul should not be demanding of CPU time on either server or clients. A spike in CPU usage could indicate too many operations taking place at once.

| Metric Name | Description |
| :---------- | :---------- |
| `cpu.user_cpu` | Percentage of CPU being used by user processes (such as Vault or Consul). |
| `cpu.iowait_cpu` | Percentage of CPU time spent waiting for I/O tasks to complete. |

If `cpu.iowait_cpu` is greater than 10%, it should be considered critical as Consul is waiting for data to be written to disk. This could be a sign that Raft is writing snapshots to disk too often. Below is an example of a healthy CPU metric.

```sh
cpu, cpu=cpu-total, usage_idle=99.298, usage_user=0.400, usage_system=0.300, usage_iowait=0, usage_steal=0 
```

### Network Activity

Network activity should be consistent. A sudden spike in network traffic to Consul might be the result of a misconfigured client, such as Vault, that is causing too many requests.

Most agents will report separate metrics for each network interface, so be sure you are monitoring the right one.

| Metric Name | Description |
| :---------- | :---------- |
| `net.bytes_recv` | Bytes received on each network interface. |
| `net.bytes_sent` | Bytes transmitted on each network interface. |

Sudden increases to the `net` metrics, greater than 50% deviation from baseline, indicates too many requests that are not being handled. Below is an example of a network activity metric.

```sh
net, interface=enp0s5, bytes_sent=6183357i, bytes_recv=262313256i
```

Note: The `net` metrics are counters, so in order to calculate rates, such as bytes/second,
you will need to apply a function such as [non_negative_difference](https://docs.influxdata.com/influxdb/v1.5/query_language/functions/#non-negative-difference).

### Disk Activity

Normally, there is low disk activity, because Consul keeps everything in memory. If the Consul host is writing a large amount of data to disk, it could mean that Consul is under heavy write load and consequently is checkpointing Raft snapshots to disk frequently. It could also mean that debug/trace logging has accidentally been enabled in production, which can impact performance. 

| Metric Name | Description |
| :---------- | :---------- |
| `diskio.read_bytes` | Bytes read from each block device. |
| `diskio.write_bytes` | Bytes written to each block device. |
| `diskio.read_time` | Time spent reading from disk, in cumulative milliseconds. |
| `diskio.write_time` | Time spent writing to disk, in cumulative milliseconds. |


Sudden, large changes to the `diskio` metrics, greater than 50% deviation from baseline
or more than 3 standard deviations from baseline indicates Consul has too much disk I/O. Too much disk I/O can cause the rest of the system to slow down or become unavailable, as the kernel spends all its time waiting for I/O to complete. Below are examples of disk activity metrics.

```sh
diskio, name=sda5, read_bytes=522298368i,  write_bytes=1726865408i, read_time=7248i, write_time=133364i
```

Note: The `diskio` metrics are counters, so in order to calculate rates (such as bytes/second),you will need to apply a function such as [non_negative_difference][].

### Memory Usage

As noted previously, Consul keeps all of its data -- the KV store, the catalog, etc -- in memory. If Consul consumes all available memory, it will crash. You should monitor total available RAM to make sure some RAM is available for other system processes and swap usage should remain at 0% for best performance.

| Metric Name | Description |
| :---------- | :---------- |
| `consul.runtime.alloc_bytes` | Measures the number of bytes allocated by the Consul process. |
| `consul.runtime.sys_bytes`   | The total number of bytes of memory obtained from the OS.  |
| `mem.total`                  | Total amount of physical memory (RAM) available on the server.     |
| `mem.used_percent`           | Percentage of physical memory in use. |
| `swap.used_percent`          | Percentage of swap space in use. |

Consul servers are running low on memory if `sys_bytes` exceeds 90% of `total_bytes`, `mem.used_percent` is over 90%, or `swap.used_percent` is greater than 0. You should increase the memory available to Consul if any of these three conditions are met. Below are examples of memory usage metrics.

```sh
'consul.runtime.alloc_bytes': 11199928.000
'consul.runtime.sys_bytes': 24627448.000
mem,  used_percent=31.492,  total=1036312576i
swap, used_percent=1.343
```
  
## Summary

In this guide we reviewed the three methods for collecting metrics. SIGUSR1 and agent HTTP API are both quick methods for collecting metrics, but enabling telemetry is the best method for moving data into monitoring software. Additionally, we outlined the various metrics collected and their significance.

