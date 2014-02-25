---
layout: "docs"
page_title: "External Services"
sidebar_current: "docs-guides-external"
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

    $ curl -X PUT -d '{"Datacenter": "dc1", "Node": "google", "Address": "www.google.com",
    "Service": {"Service": "search", "Port": 80}}' http://127.0.0.1:8500/v1/catalog/register

If we do a DNS lookup now, we can see the new search service:

    $ dig @127.0.0.1 -p 8600 search.service.consul. ANY

    ; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 search.service.consul. ANY
    ; (1 server found)
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 46641
    ;; flags: qr aa rd; QUERY: 1, ANSWER: 2, AUTHORITY: 1, ADDITIONAL: 1
    ;; WARNING: recursion requested but not available

    ;; QUESTION SECTION:
    ;search.service.consul.		IN	ANY

    ;; ANSWER SECTION:
    search.service.consul.	0	IN	CNAME	www.google.com.
    search.service.consul.	0	IN	SRV	1 1 80 google.node.dc1.consul.

    ;; AUTHORITY SECTION:
    consul.			0	IN	SOA	ns.consul. postmaster.consul. 1393359541 3600 600 86400 0

    ;; ADDITIONAL SECTION:
    google.node.dc1.consul.	0	IN	CNAME	www.google.com.

If at any time we want to deregister the service, we can simply do:

    $ curl -X PUT -d '{"Datacenter": "dc1", "Node": "google"}' http://127.0.0.1:8500/v1/catalog/deregister

This will deregister the `google` node, along with all services it provides.
To learn more, read about the [HTTP API](/docs/agent/http.html).

