---
layout: "intro"
page_title: "Registering Health Checks"
sidebar_current: "gettingstarted-checks"
---

# Registering Health Checks

We've already seen how simple registering a service is. In this section we will
continue by adding both a service level health check, as well as a host level
health check.

## Defining Checks

Similarly to a service, a check can be registered either by providing a
[check definition](/docs/agent/checks.html), or by making the appropriate calls to the
[HTTP API](/docs/agent/http.html). We will use a simple check definition to get started.
On the second node, we start by adding some additional configuration:

```
$ echo '{"check": {"name": "ping", "script": "ping -c1 google.com >/dev/null", "interval": "30s"}}' | sudo tee /etc/consul/ping.json

$ echo '{"service": {"name": "web", "tags": ["rails"], "port": 80,
  "check": {"script": "curl localhost:80 >/dev/null 2>&1", "interval": "10s"}}}' | sudo tee /etc/consul/web.json
```

The first command adds a "ping" check. This check runs on a 30 second interval, invoking
the "ping -c1 google.com" command. The second command is modifying our previous definition of
the `web` service to include a check. This check uses curl every 10 seconds to verify that
our web server is running.

We now restart the second agent, with the same parameters as before. We should now see the following
log lines:

```
==> Starting Consul agent...
...
    [INFO] agent: Synced service 'web'
    [INFO] agent: Synced check 'service:web'
    [INFO] agent: Synced check 'ping'
    [WARN] Check 'service:web' is now critical
```

The first few log lines indicate that the agent has synced the new checks and service updates
with the Consul servers. The last line indicates that the check we added for the `web` service
is critical. This is because we are not actually running a web server and the curl test
we've added is failing!

## Checking Health Status

Now that we've added some simple checks, we can use the HTTP API to check them. First,
we can look for any failing checks:

```
$ curl http://localhost:8500/v1/health/state/critical
[{"Node":"agent-two","CheckID":"service:web","Name":"Service 'web' check","Status":"critical","Notes":"","ServiceID":"web","ServiceName":"web"}]
```

We can see that there is only a single check in the `critical` state, which is our
`web` service check. If we try to perform a DNS lookup for the service, we will see that
we don't get any results:

```
 dig @127.0.0.1 -p 8600 web.service.consul

; <<>> DiG 9.8.1-P1 <<>> @127.0.0.1 -p 8600 web.service.consul
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 35753
;; flags: qr aa rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;web.service.consul.		IN	A
```

The DNS interface uses the health information and avoids routing to nodes that
are failing their health checks. This is all managed for us automatically.

This section should have shown that checks can be easily added. Check definitions
can be updated by changing configuration files and sending a `SIGHUP` to the agent.
Alternatively the HTTP API can be used to add, remove and modify checks dynamically.
The API allows allows for a "dead man's switch" or [TTL based check](/docs/agent/checks.html).
TTL checks can be used to integrate an application more tightly with Consul, enabling
business logic to be evaluated as part of passing a check.

