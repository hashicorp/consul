---
layout: "intro"
page_title: "Registering Services"
sidebar_current: "gettingstarted-services"
---

# Registering Services

In the previous page, we created a simple cluster. Although the cluster members
could see each other, there were no registered services. In this page, we'll
modify our client to export a service.


## Defining a Service

A service can be registered either by providing a [service definition](/docs/agent/services.html),
or by making the appropriate calls to the [HTTP API](/docs/agent/http.html). First we
start by providing a simple service definition. We will by using the same setup as in the
[last page](/intro/getting-started/join.html). On the second node, we start by creating a
simple configuration:

```
$ sudo mkdir /etc/consul
$ echo '{"service": {"name": "web", "tags": ["rails"], "port": 80}}' | sudo tee /etc/consul/web.json
```

We now restart the second agent, providing the configuration directory as well as the
first node to re-join:

```
$ consul agent -data-dir /tmp/consul -node=agent-two -bind=172.20.20.11 -config-dir /etc/consul/
==> Starting Consul agent...
...
    [INFO] agent: Synced service 'web'
...
```


## Querying Services

Once the agent gets started, we should see a log output indicating that the `web` service
has been synced with the Consul servers. We can first check using the HTTP API:

```
$ curl http://localhost:8500/v1/catalog/service/web
[{"Node":"agent-two","Address":"172.20.20.11","ServiceID":"web","ServiceName":"web","ServiceTags":["rails"],"ServicePort":80}]
```

We can also do a simple DNS lookup for any nodes providing the `web` service:

```
$ dig @127.0.0.1 -p 8600 web.service.consul

; <<>> DiG 9.8.1-P1 <<>> @127.0.0.1 -p 8600 web.service.consul
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 1204
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;web.service.consul.		IN	A

;; ANSWER SECTION:
web.service.consul.	0	IN	A	172.20.20.11
```

We can also filter on tags, here only requesting services matching the `rails` tag,
and specifically requesting SRV records:

```
$ dig @127.0.0.1 -p 8600 rails.web.service.consul SRV

; <<>> DiG 9.8.1-P1 <<>> @127.0.0.1 -p 8600 rails.web.service.consul SRV
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 45798
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;rails.web.service.consul.	IN	SRV

;; ANSWER SECTION:
rails.web.service.consul. 0	IN	SRV	1 1 80 agent-two.node.dc1.consul.

;; ADDITIONAL SECTION:
agent-two.node.dc1.consul. 0	IN	A	172.20.20.11
```

This shows how simple it is to get started with services. Service definitions
can be updated by changing configuration files and sending a `SIGHUP` to the agent.
Alternatively the HTTP API can be used to add, remove and modify services dynamically.

