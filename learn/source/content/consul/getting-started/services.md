---
name: 'Registering Services'
content_length: 6
layout: content_layout
description: |-
  A service can be registered with Consul either by providing a service definition or by making the appropriate calls to the HTTP API. In this guide we will register a service with a configuration file.
id: services
products_used:
  - Consul
level: Beginner
wistia_video_id: seawr6n5dk
---

In the previous step we ran our first agent, saw the cluster members (well,
our cluster _member_), and queried that node. In this guide, we'll register
our first service and query that service.

## Defining a Service

A service can be registered either by providing a
[service definition](https://www.consul.io/docs/agent/services.html) or by making the appropriate
calls to the [HTTP API](https://www.consul.io/api/agent/service.html#register-service).

A service definition is the most common way to register services, so we'll
use that approach for this step. We'll be building on the agent configuration
we covered in the [previous step](/consul/getting-started/agent).

First, create a directory for Consul configuration. Consul loads all
configuration files in the configuration directory, so a common convention
on Unix systems is to name the directory something like `/etc/consul.d`
(the `.d` suffix implies "this directory contains a set of configuration
files").

```text
$ mkdir ./consul.d
```

Next, we'll write a service definition configuration file. Let's
pretend we have a service named "web" running on port 80. Additionally,
we'll give it a tag we can use as an additional way to query the service:

```text
$ echo '{"service": {"name": "web", "tags": ["rails"], "port": 80}}' \
    > ./consul.d/web.json
```

Now, restart the agent, providing the configuration directory:

```text
$ consul agent -dev -config-dir=./consul.d

==> Starting Consul agent...
...
    [INFO] agent: Synced service 'web'
...
```

You'll notice in the output that it "synced" the web service. This means
that the agent loaded the service definition from the configuration file,
and has successfully registered it in the service catalog.

If you wanted to register multiple services, you could create multiple
service definition files in the Consul configuration directory.

~> NOTE: In a production environment, you should enable service health checks
and start a healthy service at port `80`. In order to simplify this exercise, we
have not done either.

## Querying Services

Once the agent is started and the service is synced, we can query the
service using either the DNS or HTTP API.

### DNS API

Let's first query our service using the DNS API. For the DNS API, the
DNS name for services is `NAME.service.consul`. By default, all DNS names
are always in the `consul` namespace, though
[this is configurable](https://consul.io/docs/agent/options.html#domain). The `service`
subdomain tells Consul we're querying services, and the `NAME` is the name
of the service.

For the web service we registered, these conventions and settings yield a
fully-qualified domain name of `web.service.consul`:

```text
$ dig @127.0.0.1 -p 8600 web.service.consul

;; QUESTION SECTION:
;web.service.consul.		IN	A

;; ANSWER SECTION:
web.service.consul.	0	IN	A	127.0.0.1
```

As you can see, an `A` record was returned with the IP address of the node on
which the service is available. `A` records can only hold IP addresses.

~> Since we started `consul` with a minimal configuration, the `A` record will
return `127.0.0.1`. See the Consul agent `-advertise` argument or the `address`
field in the [service
definition](https://www.consul.io/docs/agent/services.html) if you want to
advertise an IP address that is meaningful to other nodes in the cluster.

You can also use the DNS API to retrieve the entire address/port pair as a
`SRV` record:

```text
$ dig @127.0.0.1 -p 8600 web.service.consul SRV

;; QUESTION SECTION:
;web.service.consul.		IN	SRV

;; ANSWER SECTION:
web.service.consul.	0	IN	SRV	1 1 80 Armons-MacBook-Air.node.dc1.consul.

;; ADDITIONAL SECTION:
Armons-MacBook-Air.node.dc1.consul. 0 IN A	127.0.0.1
```

The `SRV` record says that the web service is running on port 80 and exists on
the node `Armons-MacBook-Air.node.dc1.consul.`. An additional section is returned by the
DNS with the `A` record for that node.

Finally, we can also use the DNS API to filter services by tags. The
format for tag-based service queries is `TAG.NAME.service.consul`. In
the example below, we ask Consul for all web services with the "rails"
tag. We get a successful response since we registered our service with
that tag:

```text
$ dig @127.0.0.1 -p 8600 rails.web.service.consul

;; QUESTION SECTION:
;rails.web.service.consul.		IN	A

;; ANSWER SECTION:
rails.web.service.consul.	0	IN	A	127.0.0.1
```

### HTTP API

In addition to the DNS API, the HTTP API can be used to query services:

```text
$ curl http://localhost:8500/v1/catalog/service/web

[{"Node":"Armons-MacBook-Air","Address":"172.20.20.11","ServiceID":"web", \
	"ServiceName":"web","ServiceTags":["rails"],"ServicePort":80}]
```

The catalog API gives all nodes hosting a given service. As we will see later
with [health checks](/consul/getting-started/checks) you'll typically want
to query just for healthy instances where the checks are passing. This is what
DNS is doing under the hood. Here's a query to look for only healthy instances:

```text
$ curl 'http://localhost:8500/v1/health/service/web?passing'

[{"Node":"Armons-MacBook-Air","Address":"172.20.20.11","Service":{ \
	"ID":"web", "Service":"web", "Tags":["rails"],"Port":80}, "Checks": ...}]
```

## Updating Services

Service definitions can be updated by changing configuration files and
sending a `SIGHUP` to the agent. This lets you update services without
any downtime or unavailability to service queries.

Alternatively, the HTTP API can be used to add, remove, and modify services
dynamically.

## Summary

We've now configured a single agent and registered a service. Other service
definition fields can be found in the [API
docs](https://www.consul.io/api/agent/service.html). This is good progress,
but let's explore the full value of Consul by learning how to automatically
encrypt and authorize service-to service communication with Consul Connect.
