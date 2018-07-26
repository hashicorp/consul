---
layout: "docs"
page_title: "Telemetry"
sidebar_current: "docs-agent-telemetry"
description: |-
  The Consul agent collects various runtime metrics about the performance of different libraries and subsystems. These metrics are aggregated on a ten second interval and are retained for one minute.
---

# Telemetry

The Consul agent collects various runtime metrics about the performance of
different libraries and subsystems. These metrics are aggregated on a ten
second interval and are retained for one minute.

To view this data, you must send a signal to the Consul process: on Unix,
this is `USR1` while on Windows it is `BREAK`. Once Consul receives the signal,
it will dump the current telemetry information to the agent's `stderr`.

This telemetry information can be used for debugging or otherwise
getting a better view of what Consul is doing.

Additionally, if the [`telemetry` configuration options](/docs/agent/options.html#telemetry)
are provided, the telemetry information will be streamed to a
[statsite](http://github.com/armon/statsite) or [statsd](http://github.com/etsy/statsd) server where
it can be aggregated and flushed to Graphite or any other metrics store. This
information can also be viewed with the [metrics endpoint](/api/agent.html#view-metrics) in JSON
format or using [Prometheus](https://prometheus.io/) format.

Below is sample output of a telemetry dump:

```text
[2014-01-29 10:56:50 -0800 PST][G] 'consul-agent.runtime.num_goroutines': 19.000
[2014-01-29 10:56:50 -0800 PST][G] 'consul-agent.runtime.alloc_bytes': 755960.000
[2014-01-29 10:56:50 -0800 PST][G] 'consul-agent.runtime.malloc_count': 7550.000
[2014-01-29 10:56:50 -0800 PST][G] 'consul-agent.runtime.free_count': 4387.000
[2014-01-29 10:56:50 -0800 PST][G] 'consul-agent.runtime.heap_objects': 3163.000
[2014-01-29 10:56:50 -0800 PST][G] 'consul-agent.runtime.total_gc_pause_ns': 1151002.000
[2014-01-29 10:56:50 -0800 PST][G] 'consul-agent.runtime.total_gc_runs': 4.000
[2014-01-29 10:56:50 -0800 PST][C] 'consul-agent.agent.ipc.accept': Count: 5 Sum: 5.000
[2014-01-29 10:56:50 -0800 PST][C] 'consul-agent.agent.ipc.command': Count: 10 Sum: 10.000
[2014-01-29 10:56:50 -0800 PST][C] 'consul-agent.serf.events': Count: 5 Sum: 5.000
[2014-01-29 10:56:50 -0800 PST][C] 'consul-agent.serf.events.foo': Count: 4 Sum: 4.000
[2014-01-29 10:56:50 -0800 PST][C] 'consul-agent.serf.events.baz': Count: 1 Sum: 1.000
[2014-01-29 10:56:50 -0800 PST][S] 'consul-agent.memberlist.gossip': Count: 50 Min: 0.007 Mean: 0.020 Max: 0.041 Stddev: 0.007 Sum: 0.989
[2014-01-29 10:56:50 -0800 PST][S] 'consul-agent.serf.queue.Intent': Count: 10 Sum: 0.000
[2014-01-29 10:56:50 -0800 PST][S] 'consul-agent.serf.queue.Event': Count: 10 Min: 0.000 Mean: 2.500 Max: 5.000 Stddev: 2.121 Sum: 25.000
```

# Key Metrics

These are some metrics emitted that can help you understand the health of your cluster at a glance. For a full list of metrics emitted by Consul, see [Metrics Reference](#metrics-reference)

### Transaction timing

| Metric Name              | Description |
| :----------------------- | :---------- |
| `consul.kvs.apply`       | This measures the time it takes to complete an update to the KV store. |
| `consul.txn.apply`       | This measures the time spent applying a transaction operation. |
| `consul.raft.apply`      | This counts the number of Raft transactions occurring over the interval. |
| `consul.raft.commitTime` | This measures the time it takes to commit a new entry to the Raft log on the leader. |

**Why they're important:** Taken together, these metrics indicate how long it takes to complete write operations in various parts of the Consul cluster. Generally these should all be fairly consistent and no more than a few milliseconds. Sudden changes in any of the timing values could be due to unexpected load on the Consul servers, or due to problems on the servers themselves.

**What to look for:** Deviations (in any of these metrics) of more than 50% from baseline over the previous hour.

### Leadership changes

| Metric Name | Description |
| :---------- | :---------- |
| `consul.raft.leader.lastContact` | Measures the time since the leader was last able to contact the follower nodes when checking its leader lease. |
| `consul.raft.state.candidate` | This increments whenever a Consul server starts an election. |
| `consul.raft.state.leader` | This increments whenever a Consul server becomes a leader. |

**Why they're important:** Normally, your Consul cluster should have a stable leader. If there are frequent elections or leadership changes, it would likely indicate network issues between the Consul servers, or that the Consul servers themselves are unable to keep up with the load.

**What to look for:** For a healthy cluster, you're looking for a `lastContact` lower than 200ms, `leader` > 0 and `candidate` == 0. Deviations from this might indicate flapping leadership.

### Autopilot

| Metric Name | Description |
| :---------- | :---------- |
| `consul.autopilot.healthy` | This tracks the overall health of the local server cluster. If all servers are considered healthy by Autopilot, this will be set to 1. If any are unhealthy, this will be 0. |

**Why it's important:** Obviously, you want your cluster to be healthy.

**What to look for:** Alert if `healthy` is 0.

### Memory usage

| Metric Name | Description |
| :---------- | :---------- |
| `consul.runtime.alloc_bytes` | This measures the number of bytes allocated by the Consul process. |
| `consul.runtime.sys_bytes`   | This is the total number of bytes of memory obtained from the OS.  |

**Why they're important:** Consul keeps all of its data in memory. If Consul consumes all available memory, it will crash.

**What to look for:** If `consul.runtime.sys_bytes` exceeds 90% of total avaliable system memory.

### Garbage collection

| Metric Name | Description |
| :---------- | :---------- |
| `consul.runtime.total_gc_pause_ns` | Number of nanoseconds consumed by stop-the-world garbage collection (GC) pauses since Consul started. |

**Why it's important:** GC pause is a "stop-the-world" event, meaning that all runtime threads are blocked until GC completes. Normally these pauses last only a few nanoseconds. But if memory usage is high, the Go runtime may GC so frequently that it starts to slow down Consul.

**What to look for:** Warning if `total_gc_pause_ns` exceeds 2 seconds/minute, critical if it exceeds 5 seconds/minute.

**NOTE:** `total_gc_pause_ns` is a cumulative counter, so in order to calculate rates (such as GC/minute),
you will need to apply a function such as InfluxDB's [`non_negative_difference()`](https://docs.influxdata.com/influxdb/v1.5/query_language/functions/#non-negative-difference).

### Network activity - RPC Count

| Metric Name | Description |
| :---------- | :---------- |
| `consul.client.rpc` | Increments whenever a Consul agent in client mode makes an RPC request to a Consul server |
| `consul.client.rpc.exceeded` | Increments whenever a Consul agent in client mode makes an RPC request to a Consul server gets rate limited by that agent's [`limits`](/docs/agent/options.html#limits) configuration.  |
| `consul.client.rpc.failed` | Increments whenever a Consul agent in client mode makes an RPC request to a Consul server and fails.  |

**Why they're important:** These measurements indicate the current load created from a Consul agent, including when the load becomes high enough to be rate limited. A high RPC count, especially from `consul.client.rpcexceeded` meaning that the requests are being rate-limited, could imply a misconfigured Consul agent.

**What to look for:**
Sudden large changes to the `consul.client.rpc` metrics (greater than 50% deviation from baseline).
`consul.client.rpc.exceeded` or `consul.client.rpc.failed` count > 0, as it implies that an agent is being rate-limited or fails to make an RPC request to a Consul server

When telemetry is being streamed to an external metrics store, the interval is defined to
be that store's flush interval. Otherwise, the interval can be assumed to be 10 seconds
when retrieving metrics from the built-in store using the above described signals.

## Metrics Reference

This is a full list of metrics emitted by Consul.

<table class="table table-bordered table-striped">
  <tr>
    <th>Metric</th>
    <th>Description</th>
    <th>Unit</th>
    <th>Type</th>
  </tr>
  <tr>
    <td>`consul.acl.blocked.service.registration`</td>
    <td>This increments whenever a deregistration fails for a service (blocked by an ACL)</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.acl.blocked.&lt;check|node|service&gt;.registration`</td>
    <td>This increments whenever a registration fails for an entity (check, node or service) is blocked by an ACL</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc`</td>
    <td>This increments whenever a Consul agent in client mode makes an RPC request to a Consul server. This gives a measure of how much a given agent is loading the Consul servers. Currently, this is only generated by agents in client mode, not Consul servers.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.exceeded`</td>
    <td>This increments whenever a Consul agent in client mode makes an RPC request to a Consul server gets rate limited by that agent's [`limits`](/docs/agent/options.html#limits) configuration. This gives an indication that there's an abusive application making too many requests on the agent, or that the rate limit needs to be increased. Currently, this only applies to agents in client mode, not Consul servers.</td>
    <td>rejected requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.failed`</td>
    <td>This increments whenever a Consul agent in client mode makes an RPC request to a Consul server and fails.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.catalog_register.<node>`</td>
    <td>This increments whenever a Consul agent receives a catalog register request.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.success.catalog_register.<node>`</td>
    <td>This increments whenever a Consul agent successfully responds to a catalog register request.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.error.catalog_register.<node>`</td>
    <td>This increments whenever a Consul agent receives an RPC error for a catalog register request.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.catalog_deregister.<node>`</td>
    <td>This increments whenever a Consul agent receives a catalog de-register request.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.success.catalog_deregister.<node>`</td>
    <td>This increments whenever a Consul agent successfully responds to a catalog de-register request.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.error.catalog_deregister.<node>`</td>
    <td>This increments whenever a Consul agent receives an RPC error for a catalog de-register request.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.catalog_datacenters.<node>`</td>
    <td>This increments whenever a Consul agent receives a request to list datacenters in the catalog.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.success.catalog_datacenters.<node>`</td>
    <td>This increments whenever a Consul agent successfully responds to a request to list datacenters.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.error.catalog_datacenters.<node>`</td>
    <td>This increments whenever a Consul agent receives an RPC error for a request to list datacenters.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.catalog_nodes.<node>`</td>
    <td>This increments whenever a Consul agent receives a request to list nodes from the catalog.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.success.catalog_nodes.<node>`</td>
    <td>This increments whenever a Consul agent successfully responds to a request to list nodes.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.error.catalog_nodes.<node>`</td>
    <td>This increments whenever a Consul agent receives an RPC error for a request to list nodes.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.catalog_services.<node>`</td>
    <td>This increments whenever a Consul agent receives a request to list services from the catalog.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.success.catalog_services.<node>`</td>
    <td>This increments whenever a Consul agent successfully responds to a request to list services.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.error.catalog_services.<node>`</td>
    <td>This increments whenever a Consul agent receives an RPC error for a request to list services.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.catalog_service_nodes.<node>`</td>
    <td>This increments whenever a Consul agent receives a request to list nodes offering a service.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.success.catalog_service_nodes.<node>`</td>
    <td>This increments whenever a Consul agent successfully responds to a request to list nodes offering a service.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.error.catalog_service_nodes.<node>`</td>
    <td>This increments whenever a Consul agent receives an RPC error for a request to list nodes offering a service.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.catalog_node_services.<node>`</td>
    <td>This increments whenever a Consul agent receives a request to list services registered in a node.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.api.success.catalog_node_services.<node>`</td>
    <td>This increments whenever a Consul agent successfully responds to a request to list services in a service.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.client.rpc.error.catalog_node_services.<node>`</td>
    <td>This increments whenever a Consul agent receives an RPC error for a request to list services in a service.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.runtime.num_goroutines`</td>
    <td>This tracks the number of running goroutines and is a general load pressure indicator. This may burst from time to time but should return to a steady state value.</td>
    <td>number of goroutines</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.runtime.alloc_bytes`</td>
    <td>This measures the number of bytes allocated by the Consul process. This may burst from time to time but should return to a steady state value.</td>
    <td>bytes</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.runtime.heap_objects`</td>
    <td>This measures the number of objects allocated on the heap and is a general memory pressure indicator. This may burst from time to time but should return to a steady state value.</td>
    <td>number of objects</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.acl.cache_hit`</td>
    <td>The number of ACL cache hits.</td>
    <td>hits</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.acl.cache_miss`</td>
    <td>The number of ACL cache misses.</td>
    <td>misses</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.acl.replication_hit`</td>
    <td>The number of ACL replication cache hits (when not running in the ACL datacenter).</td>
    <td>hits</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.dns.stale_queries`</td>
    <td>This increments when an agent serves a query within the allowed stale threshold.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.dns.ptr_query.<node>`</td>
    <td>This measures the time spent handling a reverse DNS query for the given node.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.dns.domain_query.<node>`</td>
    <td>This measures the time spent handling a domain query for the given node.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.http.<verb>.<path>`</td>
    <td>This tracks how long it takes to service the given HTTP request for the given verb and path. Paths do not include details like service or key names, for these an underscore will be present as a placeholder (eg. `consul.http.GET.v1.kv._`)</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
</table>

## Server Health

These metrics are used to monitor the health of the Consul servers.

<table class="table table-bordered table-striped">
  <tr>
    <th>Metric</th>
    <th>Description</th>
    <th>Unit</th>
    <th>Type</th>
  </tr>
  <tr>
    <td>`consul.raft.fsm.snapshot`</td>
    <td>This metric measures the time taken by the FSM to record the current state for the snapshot.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.fsm.apply`</td>
    <td>This metric gives the number of logs committed since the last interval. </td>
    <td>commit logs / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.fsm.restore`</td>
    <td>This metric measures the time taken by the FSM to restore its state from a snapshot.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.snapshot.create`</td>
    <td>This metric measures the time taken to initialize the snapshot process.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.snapshot.persist`</td>
    <td>This metric measures the time taken to dump the current snapshot taken by the Consul agent to the disk.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.snapshot.takeSnapshot`</td>
    <td>This metric measures the total time involved in taking the current snapshot (creating one and persisting it) by the Consul agent.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.replication.heartbeat`</td>
    <td>This metric measures the time taken to invoke appendEntries on a peer, so that it doesn’t timeout on a periodic basis.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.serf.snapshot.appendLine`</td>
    <td>This metric measures the time taken by the Consul agent to append an entry into the existing log.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.serf.snapshot.compact`</td>
    <td>This metric measures the time taken by the Consul agent to compact a log. This operation occurs only when the snapshot becomes large enough to justify the compaction .</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.state.leader`</td>
    <td>This increments whenever a Consul server becomes a leader. If there are frequent leadership changes this may be indication that the servers are overloaded and aren't meeting the soft real-time requirements for Raft, or that there are networking problems between the servers.</td>
    <td>leadership transitions / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.state.candidate`</td>
    <td>This increments whenever a Consul server starts an election. If this increments without a leadership change occurring it could indicate that a single server is overloaded or is experiencing network connectivity issues.</td>
    <td>election attempts / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.apply`</td>
    <td>This counts the number of Raft transactions occurring over the interval, which is a general indicator of the write load on the Consul servers.</td>
    <td>raft transactions / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.barrier`</td>
    <td>This metric counts the number of times the agent has started the barrier i.e the number of times it has
    issued a blocking call, to ensure that the agent has all the pending operations that were queued, to be applied to the agent's FSM.</td>
    <td>blocks / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.verify_leader`</td>
    <td>This metric counts the number of times an agent checks whether it is still the leader or not</td>
    <td>checks / interval</td>
    <td>Counter</td>
  </tr>
  <tr>
    <td>`consul.raft.restore`</td>
    <td>This metric counts the number of times the restore operation has been performed by the agent. Here, restore refers to the action of raft consuming an external snapshot to restore its state.</td>
    <td>operation invoked / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.commitTime`</td>
    <td>This measures the time it takes to commit a new entry to the Raft log on the leader.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.leader.dispatchLog`</td>
    <td>This measures the time it takes for the leader to write log entries to disk.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.replication.appendEntries`</td>
    <td>This measures the time it takes to replicate log entries to followers. This is a general indicator of the load pressure on the Consul servers, as well as the performance of the communication between the servers.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.state.follower`</td>
    <td>This metric counts the number of times an agent has entered the follower mode. This happens when a new agent joins the cluster or after the end of a leader election.</td>
    <td> follower state entered / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.transistion.heartbeat_timeout`</td>
    <td>This metric gives the number of times an agent has transitioned to the Candidate state, after receive no heartbeat messages from the last known leader.</td>
    <td>timeouts / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.raft.restoreUserSnapshot`</td>
    <td>This metric measures the time taken by the agent to restore the FSM state from a user's snapshot</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.rpc.processHeartBeat`</td>
    <td>This metric measures the time taken to process a heartbeat request.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.rpc.appendEntries`</td>
    <td>This metric measures the time taken to process an append entries RPC call from an agent.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.rpc.appendEntries.storeLogs`</td>
    <td>This metric measures the time taken to add any outstanding logs for an agent, since the last appendEntries was invoked</td>  
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.rpc.appendEntries.processLogs`</td>
    <td>This metric measures the time taken to process the outstanding log entries of an agent.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.rpc.requestVote`</td>
    <td>This metric measures the time taken to process the request vote RPC call.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.rpc.installSnapshot`</td>
    <td>This metric measures the time taken to process the installSnapshot RPC call. This metric should only be seen on agents which are currently in the follower state.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.replication.appendEntries.rpc`</td>
    <td>This metric measures the time taken by the append entries RFC, to replicate the log entries of a leader agent onto its follower agent(s)</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.raft.replication.appendEntries.logs`</td>
    <td>This metric measures the number of logs replicated to an agent, to bring it upto speed with the leader's logs.</td>
    <td>logs appended/ interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td><a name="last-contact"></a>`consul.raft.leader.lastContact`</td>
    <td>This will only be emitted by the Raft leader and measures the time since the leader was last able to contact the follower nodes when checking its leader lease. It can be used as a measure for how stable the Raft timing is and how close the leader is to timing out its lease.<br><br>The lease timeout is 500 ms times the [`raft_multiplier` configuration](/docs/agent/options.html#raft_multiplier), so this telemetry value should not be getting close to that configured value, otherwise the Raft timing is marginal and might need to be tuned, or more powerful servers might be needed. See the [Server Performance](/docs/guides/performance.html) guide for more details.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.acl.apply`</td>
    <td>This measures the time it takes to complete an update to the ACL store.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.acl.fault`</td>
    <td>This measures the time it takes to fault in the rules for an ACL during a cache miss.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.acl.fetchRemoteACLs`</td>
    <td>This measures the time it takes to fetch remote ACLs during replication.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.acl.updateLocalACLs`</td>
    <td>This measures the time it takes to apply replication changes to the local ACL store.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.acl.replicateACLs`</td>
    <td>This measures the time it takes to do one pass of the ACL replication algorithm.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.acl.resolveToken`</td>
    <td>This measures the time it takes to resolve an ACL token.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.rpc.accept_conn`</td>
    <td>This increments when a server accepts an RPC connection.</td>
    <td>connections</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.catalog.register`</td>
    <td>This measures the time it takes to complete a catalog register operation.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.catalog.deregister`</td>
    <td>This measures the time it takes to complete a catalog deregister operation.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.register`</td>
    <td>This measures the time it takes to apply a catalog register operation to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.deregister`</td>
    <td>This measures the time it takes to apply a catalog deregister operation to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.acl.<op>`</td>
    <td>This measures the time it takes to apply the given ACL operation to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.session.<op>`</td>
    <td>This measures the time it takes to apply the given session operation to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.kvs.<op>`</td>
    <td>This measures the time it takes to apply the given KV operation to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.tombstone.<op>`</td>
    <td>This measures the time it takes to apply the given tombstone operation to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.coordinate.batch-update`</td>
    <td>This measures the time it takes to apply the given batch coordinate update to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.prepared-query.<op>`</td>
    <td>This measures the time it takes to apply the given prepared query update operation to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.txn`</td>
    <td>This measures the time it takes to apply the given transaction update to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.autopilot`</td>
    <td>This measures the time it takes to apply the given autopilot update to the FSM.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.fsm.persist`</td>
    <td>This measures the time it takes to persist the FSM to a raft snapshot.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.kvs.apply`</td>
    <td>This measures the time it takes to complete an update to the KV store.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.leader.barrier`</td>
    <td>This measures the time spent waiting for the raft barrier upon gaining leadership.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.leader.reconcile`</td>
    <td>This measures the time spent updating the raft store from the serf member information.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.leader.reconcileMember`</td>
    <td>This measures the time spent updating the raft store for a single serf member's information.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.leader.reapTombstones`</td>
    <td>This measures the time spent clearing tombstones.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.prepared-query.apply`</td>
    <td>This measures the time it takes to apply a prepared query update.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.prepared-query.explain`</td>
    <td>This measures the time it takes to process a prepared query explain request.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.prepared-query.execute`</td>
    <td>This measures the time it takes to process a prepared query execute request.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.prepared-query.execute_remote`</td>
    <td>This measures the time it takes to process a prepared query execute request that was forwarded to another datacenter.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.rpc.raft_handoff`</td>
    <td>This increments when a server accepts a Raft-related RPC connection.</td>
    <td>connections</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.rpc.request_error`</td>
    <td>This increments when a server returns an error from an RPC request.</td>
    <td>errors</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.rpc.request`</td>
    <td>This increments when a server receives a Consul-related RPC request.</td>
    <td>requests</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.rpc.query`</td>
    <td>This increments when a server receives a (potentially blocking) RPC query.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.rpc.cross-dc`</td>
    <td>This increments when a server receives a (potentially blocking) cross datacenter RPC query.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.rpc.consistentRead`</td>
    <td>This measures the time spent confirming that a consistent read can be performed.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.session.apply`</td>
    <td>This measures the time spent applying a session update.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.session.renew`</td>
    <td>This measures the time spent renewing a session.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.session_ttl.invalidate`</td>
    <td>This measures the time spent invalidating an expired session.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.txn.apply`</td>
    <td>This measures the time spent applying a transaction operation.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.txn.read`</td>
    <td>This measures the time spent returning a read transaction.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
</table>

## Cluster Health

These metrics give insight into the health of the cluster as a whole.

<table class="table table-bordered table-striped">
  <tr>
    <th>Metric</th>
    <th>Description</th>
    <th>Unit</th>
    <th>Type</th>
  </tr>
  <tr>
    <td>`consul.memberlist.degraded.probe`</td>
    <td>This metric counts the number of times the agent has performed failure detection on an other agent at a slower probe rate. The agent uses its own health metric as an indicator to perform this action. (If its health score is low, means that the node is healthy, and vice versa.)</td>
    <td>probes / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.degraded.timeout`</td>
    <td>This metric counts the number of times an agent was marked as a dead node, whilst not getting enough confirmations from a randomly selected list of agent nodes in an agent's membership.</td>
    <td>occurrence / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.msg.dead`</td>
    <td>This metric counts the number of times an agent has marked another agent to be a dead node.</td>
    <td>messages / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.health.score`</td>
    <td>This metric describes a node's perception of its own health based on how well it is meeting the soft real-time requirements of the protocol. This metric ranges from 0 to 8, where 0 indicates "totally healthy". This health score is used to scale the time between outgoing probes, and higher scores translate into longer probing intervals. For more details see section IV of the Lifeguard paper: https://arxiv.org/pdf/1707.00788.pdf</td>
    <td>score</td>
    <td>gauge</td>
  </tr>  
  <tr>
    <td>`consul.memberlist.msg.suspect`</td>
    <td>This increments when an agent suspects another as failed when executing random probes as part of the gossip protocol. These can be an indicator of overloaded agents, network problems, or configuration errors where agents can not connect to each other on the [required ports](/docs/agent/options.html#ports).</td>
    <td>suspect messages received / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.tcp.accept`</td>
    <td>This metric counts the number of times an agent has accepted an incoming TCP stream connection.</td>
    <td>connections accepted / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.udp.sent/received`</td>
    <td>This metric measures the total number of bytes sent/received by an agent through the UDP protocol.</td>
    <td>bytes sent or bytes received / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.tcp.connect`</td>
    <td>This metric counts the number of times an agent has initiated a push/pull sync with an other agent.</td>
    <td>push/pull initiated / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.tcp.sent`</td>
    <td>This metric measures the total number of bytes sent by an agent through the TCP protocol</td>
    <td>bytes sent / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.gossip`</td>
    <td>This metric gives the number of gossips (messages) broadcasted to a set of randomly selected nodes.</td>
    <td>messages / Interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.msg_alive`</td>
    <td>This metric counts the number of alive agents, that the agent has mapped out so far, based on the message information given by the network layer.</td>
    <td>nodes / Interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.msg_dead`</td>
    <td>This metric gives the number of dead agents, that the agent has mapped out so far, based on the message information given by the network layer.</td>
    <td>nodes / Interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.msg_suspect`</td>
    <td>This metric gives the number of suspect nodes, that the agent has mapped out so far, based on the message information given by the network layer.</td>
    <td>nodes / Interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.probeNode`</td>
    <td>This metric measures the time taken to perform a single round of failure detection on a select agent.</td>
    <td>nodes / Interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.memberlist.pushPullNode`</td>
    <td>This metric measures the number of agents that have exchanged state with this agent.</td>
    <td>nodes / Interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.serf.member.flap`</td>
    <td>Available in Consul 0.7 and later, this increments when an agent is marked dead and then recovers within a short time period. This can be an indicator of overloaded agents, network problems, or configuration errors where agents can not connect to each other on the [required ports](/docs/agent/options.html#ports).</td>
    <td>flaps / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.serf.events`</td>
    <td>This increments when an agent processes an [event](/docs/commands/event.html). Consul uses events internally so there may be additional events showing in telemetry. There are also a per-event counters emitted as `consul.serf.events.<event name>`.</td>
    <td>events / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.autopilot.failure_tolerance`</td>
    <td>This tracks the number of voting servers that the cluster can lose while continuing to function.</td>
    <td>servers</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.autopilot.healthy`</td>
    <td>This tracks the overall health of the local server cluster. If all servers are considered healthy by Autopilot, this will be set to 1. If any are unhealthy, this will be 0.</td>
    <td>boolean</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.session_ttl.active`</td>
    <td>This tracks the active number of sessions being tracked.</td>
    <td>sessions</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.catalog.service.query.<service>`</td>
    <td>This increments for each catalog query for the given service.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.catalog.service.query-tag.<service>.<tag>`</td>
    <td>This increments for each catalog query for the given service with the given tag.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.catalog.service.not-found.<service>`</td>
    <td>This increments for each catalog query where the given service could not be found.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.health.service.query.<service>`</td>
    <td>This increments for each health query for the given service.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.health.service.query-tag.<service>.<tag>`</td>
    <td>This increments for each health query for the given service with the given tag.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.health.service.not-found.<service>`</td>
    <td>This increments for each health query where the given service could not be found.</td>
    <td>queries</td>
    <td>counter</td>
  </tr>
</table>


## Connect Built-in Proxy Metrics

Consul Connect's built-in proxy is by default configured to log metrics to the
same sink as the agent that starts it when running as a [managed
proxy](/docs/connect/proxies.html#managed-proxies).

When running in this mode it emits some basic metrics. These will be expanded
upon in the future.

All metrics are prefixed with `consul.proxy.<proxied-service-id>` to distinguish
between multiple proxies on a given host. The table below use `web` as an
example service name for brevity.

### Labels

Most labels have a `dst` label and some have a `src` label. When using metrics
sinks and timeseries stores that support labels or tags, these allow aggregating
the connections by service name.

Assuming all services are using a managed built-in proxy, you can get a complete
overview of both number of open connections and bytes sent and recieved between
all services by aggregating over these metrics.

For example aggregating over all `upstream` (i.e. outbound) connections which
have both `src` and `dst` labels, you can get a sum of all the bandwidth in and
out of a given service or the total number of connections between two services.


### Metrics Reference

The standard go runtime metrics are exported by `go-metrics` as with Consul
agent. The table below describes the additional metrics exported by the proxy.

<table class="table table-bordered table-striped">
  <tr>
    <th>Metric</th>
    <th>Description</th>
    <th>Unit</th>
    <th>Type</th>
  </tr>
  <tr>
    <td>`consul.proxy.web.runtime.*`</td>
    <td>The same go runtime metrics as documented for the agent above.</td>
    <td>mixed</td>
    <td>mixed</td>
  </tr>
  <tr>
    <td>`consul.proxy.web.inbound.conns`</td>
    <td>Shows the current number of connections open from inbound requests to
    the proxy. Where supported a `dst` label is added indicating the
    service name the proxy represents.</td>
    <td>connections</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.proxy.web.inbound.rx_bytes`</td>
    <td>This increments by the number of bytes received from an inbound client
    connection. Where supported a `dst` label is added indicating the
    service name the proxy represents.</td>
    <td>bytes</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.proxy.web.inbound.tx_bytes`</td>
    <td>This increments by the number of bytes transfered to an inbound client
    connection. Where supported a `dst` label is added indicating the
    service name the proxy represents.</td>
    <td>bytes</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.proxy.web.upstream.conns`</td>
    <td>Shows the current number of connections open from a proxy instance to an
    upstream. Where supported a `src` label is added indicating the
    service name the proxy represents, and a `dst` label is added indicating the
    service name the upstream is connecting to.</td>
    <td>connections</td>
    <td>gauge</td>
  </tr>
  <tr>
    <td>`consul.proxy.web.inbound.rx_bytes`</td>
    <td>This increments by the number of bytes received from an upstream
    connection. Where supported a `src` label is added indicating the
    service name the proxy represents, and a `dst` label is added indicating the
    service name the upstream is connecting to.</td>
    <td>bytes</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.proxy.web.inbound.tx_bytes`</td>
    <td>This increments by the number of bytes transfered to an upstream
    connection. Where supported a `src` label is added indicating the
    service name the proxy represents, and a `dst` label is added indicating the
    service name the upstream is connecting to.</td>
    <td>bytes</td>
    <td>counter</td>
  </tr>
</table>
