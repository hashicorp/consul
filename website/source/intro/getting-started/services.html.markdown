---
layout: "intro"
page_title: "Registering Services"
sidebar_current: "gettingstarted-services"
description: |-
  In the previous page, we ran our first agent, saw the cluster members, and queried that node. On this page, we'll register our first service and query that service. We're not yet running a cluster of Consul agents.
---

# Registering Services

In the previous page, we ran our first agent, saw the cluster members, and
queried that node. On this page, we'll register our first service and query
that service. We're not yet running a cluster of Consul agents.

## Defining a Service

A service can be registered either by providing a
[service definition](/docs/agent/services.html),
or by making the appropriate calls to the
[HTTP API](/docs/agent/http.html).

We're going to start by registering a service using a service definition,
since this is the most common way that services are registered. We'll be
building on what we covered in the
[previous page](/intro/getting-started/agent.html).

First, create a directory for Consul configurations. A good directory
is typically `/etc/consul.d`. Consul loads all configuration files in the
configuration directory.

```text
$ sudo mkdir /etc/consul.d
```

Next, we'll write a service definition configuration file. We'll
pretend we have a service named "web" running on port 80. Additionally,
we'll give it some tags, which we can use as additional ways to query
it later.

```text
$ echo '{"service": {"name": "web", "tags": ["rails"], "port": 80}}' \
    >/etc/consul.d/web.json
```

Now, restart the agent we're running, providing the configuration directory:

```text
$ consul agent -server -bootstrap-expect 1 -data-dir /tmp/consul -config-dir /etc/consul.d
==> Starting Consul agent...
...
    [INFO] agent: Synced service 'web'
...
```

You'll notice in the output that it "synced" the web service. This means
that it loaded the information from the configuration.

If you wanted to register multiple services, you create multiple service
definition files in the Consul configuration directory.

## Querying Services

Once the agent is started and the service is synced, we can query that
service using either the DNS or HTTP API.

### DNS API

Let's first query it using the DNS API. For the DNS API, the DNS name
for services is `NAME.service.consul`. All DNS names are always in the
`consul` namespace. The `service` subdomain tells Consul we're querying
services, and the `NAME` is the name of the service. For the web service
we registered, that would be `web.service.consul`:

```text
$ dig @127.0.0.1 -p 8600 web.service.consul
...

;; QUESTION SECTION:
;web.service.consul.		IN	A

;; ANSWER SECTION:
web.service.consul.	0	IN	A	172.20.20.11
```

As you can see, an A record was returned with the IP address of the node that
the service is available on. A records can only hold IP addresses. You can
also use the DNS API to retrieve the entire address/port pair using SRV
records:

```text
$ dig @127.0.0.1 -p 8600 web.service.consul SRV
...

;; QUESTION SECTION:
;web.service.consul.	IN	SRV

;; ANSWER SECTION:
web.service.consul. 0	IN	SRV	1 1 80 agent-one.node.dc1.consul.

;; ADDITIONAL SECTION:
agent-one.node.dc1.consul. 0	IN	A	172.20.20.11
```

The SRV record returned says that the web service is running on port 80
and exists on the node `agent-one.node.dc1.consul.`. An additional section
is returned by the DNS with the A record for that node.

Finally, we can also use the DNS API to filter services by tags. The
format for tag-based service queries is `TAG.NAME.service.consul`. In
the example below, we ask Consul for all web services with the "rails"
tag. We get a response since we registered our service with that tag.

```text
$ dig @127.0.0.1 -p 8600 rails.web.service.consul
...

;; QUESTION SECTION:
;rails.web.service.consul.		IN	A

;; ANSWER SECTION:
rails.web.service.consul.	0	IN	A	172.20.20.11
```

### HTTP API

In addition to the DNS API, the HTTP API can be used to query services:

```text
$ curl http://localhost:8500/v1/catalog/service/web
[{"Node":"agent-one","Address":"172.20.20.11","ServiceID":"web","ServiceName":"web","ServiceTags":["rails"],"ServicePort":80}]
```

## Updating Services

Service definitions can be updated by changing configuration files and
sending a `SIGHUP` to the agent. This lets you update services without
any downtime or unavailability to service queries.

Alternatively the HTTP API can be used to add, remove, and modify services
dynamically.
