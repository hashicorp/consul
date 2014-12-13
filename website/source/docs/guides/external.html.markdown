---
layout: "docs"
page_title: "External Services"
sidebar_current: "docs-guides-external"
description: |-
  Very few infrastructures are entirely self-contained, and often rely on a multitude of external service providers. Most services are registered in Consul through the use of a service definition, however that registers the local node as the service provider. In the case of external services, we want to register a service as being provided by an external provider.
---

# Registering an External Service

Very few infrastructures are entirely self-contained, and often rely on
a multitude of external service providers. Most services are registered
in Consul through the use of a [service definition](/docs/agent/services.html),
however that registers the local node as the service provider. In the case
of external services, we want to register a service as being provided by
an external provider.

Consul supports this, however it requires manually registering the service
with the catalog. Once registered, the DNS interface will be able to return
the appropriate A records or CNAME records for the service. The service will
also appear in standard queries against the API.

Let us suppose we want to register a "search" service that is provided by
"www.google.com", we could do the following:

```text
$ curl -X PUT -d '{"Datacenter": "dc1", "Node": "google", "Address": "www.google.com",
"Service": {"Service": "search", "Port": 80}}' http://127.0.0.1:8500/v1/catalog/register
```

If we do a DNS lookup now, we can see the new search service:

```text
; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 search.service.consul.
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 13313
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 4, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;search.service.consul.		IN	A

;; ANSWER SECTION:
search.service.consul.	0	IN	CNAME	www.google.com.
www.google.com.		264	IN	A	74.125.239.114
www.google.com.		264	IN	A	74.125.239.115
www.google.com.		264	IN	A	74.125.239.116

;; Query time: 41 msec
;; SERVER: 127.0.0.1#8600(127.0.0.1)
;; WHEN: Tue Feb 25 17:45:12 2014
;; MSG SIZE  rcvd: 178
```

If at any time we want to deregister the service, we can simply do:

```text
$ curl -X PUT -d '{"Datacenter": "dc1", "Node": "google"}' http://127.0.0.1:8500/v1/catalog/deregister
```

This will deregister the `google` node, along with all services it provides.
To learn more, read about the [HTTP API](/docs/agent/http.html).
