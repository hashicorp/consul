# prometheus

This module enables prometheus metrics for CoreDNS. The default location for the metrics is
`localhost:9153`. The metrics path is fixed to `/metrics`.

The following metrics are exported:

* coredns_dns_request_count_total
* coredns_dns_request_duration_seconds
* coredns_dns_request_size_bytes
* coredns_dns_request_do_count_total
* coredns_dns_response_size_bytes
* coredns_dns_response_rcode_count_total

Each counter has a label `zone` which is the zonename used for the request/response. and a label
`qtype` which old the query type. The `dns_request_count_total` has extra labels: `proto` which
holds the transport of the response ("udp" or "tcp") and the address family of the transport (1
= IP (IP version 4), 2 = IP6 (IP version 6)).
The `response_rcode_count_total` has an extra label `rcode` which holds the rcode of the response.
The `*_size_bytes` counters also hold the protocol in the `proto` label ("udp" or "tcp").

If monitoring is enabled queries that do not enter the middleware chain are exported under the fake
domain "dropped" (without a closing dot).

Restarting CoreDNS will stop the monitoring. This is a bug. Also [this upstream
Caddy bug](https://github.com/mholt/caddy/issues/675).

## Syntax

~~~
prometheus
~~~

For each zone that you want to see metrics for.

It optionally takes an address where the metrics are exported, the default
is `localhost:9153`. The metrics path is fixed to `/metrics`.

## Examples
