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
it can be aggregated and flushed to Graphite or any other metrics store.

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

When telemetry is being streamed to an external metrics store, the interval is defined to
be that store's flush interval. Otherwise, the interval can be assumed to be 10 seconds
when retrieving metrics from the built-in store using the above described signals.

## Agent Health

These metrics are used to monitor the health of specific Consul agents.

<table class="table table-bordered table-striped">
  <tr>
    <th>Metric</th>
    <th>Description</th>
    <th>Unit</th>
    <th>Type</th>
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
    <td>`consul.raft.leader.lastContact`</td>
    <td>This measures the time that a Consul server was last contacted by the leader (will be zero on the leader itself). This is a general indicator of latency in the Raft subsystem, and gives a general indicator of how far behind [stale](/docs/agent/http.html#consistency) queries will be.</td>
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
    <td>`consul.memberlist.msg.suspect`</td>
    <td>This increments when an agent suspects another as failed when executing random probes as part of the gossip protocol. These can be an indicator of overloaded agents, network problems, or configuration errors where agents can not connect to each other on the [required ports](/docs/agent/options.html#ports).</td>
    <td>suspect messages received / interval</td>
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
    <td>This increments when an agent processes an [event](/docs/commands/event.html). Note that Consul uses events internally so there may be additional events showing in telemetry. There are also a per-event counters emitted as `consul.serf.events.<event name>`.</td>
    <td>events / interval</td>
    <td>counter</td>
  </tr>
  <tr>
    <td>`consul.dns.domain_query.<agent>`</td>
    <td>This tracks how long it takes to service forward DNS lookups on the given Consul agent.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
  <tr>
    <td>`consul.dns.ptr_query.<agent>`</td>
    <td>This tracks how long it takes to service reverse DNS lookups on the given Consul agent.</td>
    <td>ms</td>
    <td>timer</td>
  </tr>
</table>
