---
name: 'Troubleshooting'
content_length: 15
id: /advanced-operations/troubleshooting
layout: content_layout
products_used:
  - Consul
description: Find and fix problems with your Consul cluster and supporting tools.
level: Advanced
---

Troubleshooting is a fundamental skill for any devops practitioner. With that in mind, Consul includes several tools to help you view log messages, validate configuration files, examine the service catalog, and do other debugging.

## Troubleshooting Steps

Before you start, consider following a pre-determined troubleshooting workflow so your process doesn't get in the way of the troubleshooting problems you are trying to figure out. Your team might use a workflow or you may have discovered a sequence that works for you. Established workflows such as [Observe, Orient, Decide, Act](https://en.wikipedia.org/wiki/OODA_loop) or even [Rubber duck debugging](https://en.wikipedia.org/wiki/Rubber_duck_debugging) have been developed for many years. We like to use the following four step process:

- Gather data
- Verify what works
- Solve one problem at a time
- Form a hypothesis and test it
- (Repeat)

### Gather Data

Consul and other tools generate log files, status messages, process lists, data feeds, and other useful information. Know where to find this data so you can establish what Consul knows about itself (and what you assume is happening).

### Verify What Works

Consul sits at the center of other highly complex systems. Many problems can be solved by first verifying that core systems, networking, and services are working as expected.

There's a counterpart to this as well. If you are uncertain about how a component works or if it is working at all, list that uncertainty in your notes or take the time to verify that it is operating normally.

Consul handles the service configuration, discovery, and segmentation responsibilities in your network. However, it assumes that you're using other tools to launch service instances (such as `systemd`, `kuberenetes`, or [Nomad](https://www.nomadproject.io/)). It may be that Consul is operating normally but is unable to deliver the connectivity you need because other tools are not able to launch services or keep them running.

### Solve One Problem at a Time

Consul is highly configurable, but you'll find it easier if you configure one piece at a time, verify successful operation, and then proceed to configure other components of the system.

This is true of any complex system. If necessary, build a smaller system where you can test the specific configuration options or features that seem to be operating incorrectly. Once you have verified proper syntax, correct network operation, and fully functioning microservices, then you can integrate those changes back into the main system.

### Form a Hypothesis and Test It

Once you have gathered data, verified what works, and identified one problem that you hope to solve, write down a hypothesis (theory) about what might be causing the error and how it might be fixed. Then observe the data and take action to see if your theory is correct (and if it fixes the problem).

### Repeat

More often than not, you may need to repeat some or all of this process to isolate the problem. Each observation may yield new data on the situation and possible solution.

By following a consistent process (no matter what it is), you can reduce the likelihood that your process will complicate the situation. Find the solution, learn from it, and integrate it back into your development, staging, and production deployments.

## Consul-Specific Tools

Now that you have a troubleshooting process, let's examine the tools and data available to you while operating Consul.

-> NOTE: Each of the Consul commands mentioned below can be run on a node with access to the `consul` agent. Use SSH or other container-specific commands to connect to a compute node in your datacenter.

### Gather Data: List Your Consul Architecture

When communicating to other members of your team or to support staff, it's helpful to list some details about your Consul architecture, such as:

- How are you querying Consul? (DNS, HTTP)
- Is the Consul web UI available for viewing?
- What system are you using for launching microservices? (`systemd`, `kubernetes`, `upstart`, Nomad, etc.)

### Members

The `consul members` command will list the other servers and agents that are part of the Consul cluster connected to the current agent.

```text
$ consul members

Node    Address         Status  Type    Build  Protocol  DC   Segment
laptop  127.0.0.1:8301  alive   server  1.4.0  2         dc1  <all>
```

- [Consul members documentation](https://www.consul.io/docs/commands/members.html)

### Monitor

The `consul monitor` command displays log output from the Consul agent. Other arguments can increase the amount of information given, such as `-log-level debug` or `-log-level trace` for copious amounts of log data.

```text
$ consul monitor

2019/01/25 17:48:33 [INFO] raft: Initial configuration (index=1): [{Suffrage:Voter ID:abcdef Address:127.0.0.1:8300}]
2019/01/25 17:48:33 [INFO] raft: Node at 127.0.0.1:8300 [Follower] entering Follower state (Leader: "")
2019/01/25 17:48:33 [INFO] serf: EventMemberJoin: laptop.dc1 127.0.0.1
2019/01/25 17:48:33 [INFO] serf: EventMemberJoin: laptop 127.0.0.1
2019/01/25 17:48:33 [INFO] consul: Adding LAN server laptop (Addr: tcp/127.0.0.1:8300) (DC: dc1)
2019/01/25 17:48:33 [INFO] consul: Handled member-join event for server "laptop.dc1" in area "wan"
2019/01/25 17:48:33 [ERR] agent: Failed decoding service file "services/.DS_Store": invalid character '\x00' looking for beginning of value
2019/01/25 17:48:33 [INFO] agent: Started DNS server 127.0.0.1:8600 (tcp)
2019/01/25 17:48:33 [INFO] agent: Started DNS server 127.0.0.1:8600 (udp)
2019/01/25 17:48:33 [INFO] agent: Started HTTP server on 127.0.0.1:8500 (tcp)
2019/01/25 17:48:33 [INFO] agent: Started gRPC server on 127.0.0.1:8502 (tcp)
```

- [Consul monitor documentation](https://www.consul.io/docs/commands/monitor.html)

### Validate

The `consul validate` command can be run on a single Consul configuration file or, more commonly, on an entire directory of configuration files. Basic syntax and logical correctness will be analyzed and reported upon.

This command will catch misspellings of Consul configuration keys, or the absence or misconfiguration of crucial attributes.

```text
$ consul validate /etc/consul.d/counting.json

* invalid config key serrrvice
```

~> TIP: Run `validate` on an entire directory of Consul configuration files so that the complete configuration can be analyzed.

- [Documentation for Consul validate](https://www.consul.io/docs/commands/validate.html)

### Debug

The `consul debug` command can be run on a node without any other arguments. It will log metrics, logs, profiling data, and other data to the current directory for two minutes.

All content is written in plain text to a compressed archive, so do not transmit the emitted data over unencrypted channels.

However, you might find this data useful or can provide it to support staff in order to help you debug your Consul cluster.

```text
$ consul debug

==> Starting debugger and capturing static information...
     Agent Version: '1.4.0'
          Interval: '30s'
          Duration: '2m0s'
            Output: 'consul-debug-1548721978.tar.gz'
           Capture: 'metrics, logs, pprof, host, agent, cluster'
==> Beginning capture interval 2019-01-28 16:32:58.56142 -0800 PST (0)
```

Unzip the archive and view the contents.

```text
$ tar xvfz consul-debug-1548721978.tar.gz
$ tree consul-debug-1548721978

├── consul-debug-1548721978
│   ├── 1548721978
│   │   ├── consul.log
│   │   ├── goroutine.prof
│   │   ├── heap.prof
│   │   ├── metrics.json
│   │   ├── profile.prof
│   │   └── trace.out
```

- [Documentation on Consul debug](https://www.consul.io/docs/commands/debug.html)

### Health Checks

When enabled, health checks are a crucial part of the operation of the Consul cluster. Unhealthy services will not be published for discovery via standard DNS or some HTTP API calls.

The easiest way to view initial health status is by visiting the [Consul Web UI](http://localhost:8500/ui) at `http://localhost:8500/ui`.

Click through to a specific service such as the `counting` service. The status of the service on each node will be displayed. Click through to see the output of the health check.

Alternately, use the HTTP API to view the entire catalog or a specific service. The `/v1/agent/services` endpoint will return a list of all services registered in the catalog.

```text
$ curl "http://127.0.0.1:8500/v1/agent/services"
```

Filters can be provided on the query string, such as this command which looks for `counting` services that are `passing` (healthy):

```text
$ curl 'http://localhost:8500/v1/health/service/counting?passing'

[
  {
    "Node": {
      "ID": "da8eb9d3-...",
      "Node": "laptop",
      "Address": "127.0.0.1",
      "Datacenter": "dc1",
    },
    "Checks": [
      {
        "Node": "laptop",
        "Output": "Agent alive and reachable"
      },
      {
        "Node": "laptop",
        "Name": "Service 'counting' check",
        "Status": "passing",
        "Output": "HTTP GET http://localhost:9003/health: 200 OK Output: Hello, you've hit /health\n",
        "ServiceName": "counting"
      }
    ]
  }
]
```

Another important aspect of health checks is not only the presence of a healthy service, but the _continued_ presence of that healthy service. If a service is oscillating between healthy and unhealthy, we call that _flapping_. It may be due to problems inherent to the service itself (an internal crash) and the service itself should be investigated (or the process launcher that runs the process, such as `systemd`).

Also consider the type of health check being run. Consider using built-in health checks such as TCP or HTTP rather than a script check which runs an external shell command to verify service health.

- [Documentation on Consul health checks](https://www.consul.io/docs/agent/checks.html)

## External Tools

Consul was designed to work within an established ecosystem of networking protocols which means that you can use existing tools to gather data and debug the network, applications, and security context surrounding Consul.

### ps

Consul service discovery relies on existing process launching tools such as `systemd` or `upstart` or `init.d`. If you choose to use one of these tools, you should refer to their documentation to learn how to write configuration files, start/stop/restart scripts, and monitor the output of processes launched by these tools.

A common tool on Unix systems is `ps`. Run it to verify that your service processes are running as expected.

```text
$ ps | grep counting

79846 ttys001    0:00.07 ./counting-service
```

Or, consider `pstree` which can be installed separately with your operating system's package manager. It displays a hierarchy of running child processes and parent processes.

```text
$ pstree

 | | \-+= 74259 geoffrey -zsh
 | |   \--= 79846 geoffrey ./counting-service
```

### dig

Given that Consul speaks the DNS protocol, standard DNS tools can be used with it. However, Consul operates by default on port `8600` instead of the DNS default of `53`.

The `dig` tool is a command line application that interacts with DNS records of all kinds. To see details about a DNS record in Consul, pass the `-p 8600` flag and the IP address of the Consul server with `@127.0.0.1`.

In this example, we find the IP address of the `counting` service.

```text
$ dig @127.0.0.1 -p 8600 counting.service.consul

; <<>> DiG 9.10.6 <<>> @127.0.0.1 -p 8600 counting.service.consul
...
;; ANSWER SECTION:
counting.service.consul. 0	IN	A	192.168.0.35
```

-> NOTE: A DNS query to Consul will only return the IP address of a healthy service, not all registered services. Use the HTTP API if you want to see a list of all healthy services, or all registered services regardless of health.

Consul also provides additional information with the `SRV` (service) argument. Add `SRV` to the `dig` command to see the port number that the `counting` service operates on (shown as port `9003` below).

```text
$ dig @127.0.0.1 -p 8600 counting.service.consul SRV

;; ANSWER SECTION:
counting.service.consul. 0	IN	SRV	1 1 9003 Machine.local.node.dc1.consul.

;; ADDITIONAL SECTION:
Machine.local.node.dc1.consul. 0 IN	A	192.168.0.35
```

If you have configured [DNS forwarding](https://www.consul.io/docs/guides/forwarding.html) to integrate system DNS with Consul, you can omit the IP address and port number. However, when debugging it is often useful to be specific so you can verify that direct communication to the Consul agent is working as expected.

### curl

As simple as it may seem, the `curl` command is extremely useful for debugging any web service or discovering details about content and HTTP headers.

Use `curl` on your configured health endpoint to verify connectivity to a service. You can provide the IP address or `localhost`. You can also provide a port number and any other path information or querystring data (you can even post form data).

If you have configured [DNS forwarding](https://www.consul.io/docs/guides/forwarding.html), you may be able to use Consul-specific domain names to communicate to the service (such as `http://counting.service.consul`).

```text
$ curl http://localhost:9003/health

Hello, you've hit /health
```

-> NOTE: If a service uses Consul Connect, you may need to communicate to a service from a node in the cluster using `localhost` and the local port as configured with Consul Connect.

### ping

The `ping` command is a simple tool that verifies network connectivity to a host (if it responds to `ping`). Use an IP address to verify that one node can communicate to another node.

`ping` is also useful for viewing the latency between nodes and the reliability of packet transfer between them.

```text
$ ping 10.0.1.14

PING 10.0.1.14 (10.0.1.14): 56 data bytes
64 bytes from 10.0.1.14: icmp_seq=0 ttl=64 time=0.065 ms
64 bytes from 10.0.1.14: icmp_seq=1 ttl=64 time=0.068 ms
64 bytes from 10.0.1.14: icmp_seq=2 ttl=64 time=0.060 ms
^C
--- 10.0.1.14 ping statistics ---
3 packets transmitted, 3 packets received, 0.0% packet loss
round-trip min/avg/max/stddev = 0.060/0.064/0.068/0.003 ms
```

## Summary

You should now be able to use the Consul tools to help troubleshoot.
- View log messages with `consul monitor`
- Validate configuration files with `consul validate`
- Examine the service catalog with the HTTP API endpoint `/v1/agent/services` 

Finally, you should be able to use external tools like `dig` and `curl` to help troubleshoot Consul issues. 

