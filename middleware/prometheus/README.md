# prometheus

This module enables prometheus metrics for CoreDNS.

The following metrics are exported:

* coredns_dns_request_count_total
* coredns_dns_request_duration_seconds
* coredns_dns_response_size_bytes
* coredns_dns_response_rcode_count_total

Each counter has a label `zone` which is the zonename used for the request/response.
The `request_count` metrics has an extra label `qtype` which holds the qtype. And
`rcode_count` has an extra label which has the rcode.

Restarting CoreDNS will stop the monitoring. This is a bug. Also [this upstream
Caddy bug](https://github.com/mholt/caddy/issues/675).

## Syntax

~~~
prometheus
~~~

For each zone that you want to see metrics for.

It optionally takes an address where the metrics are exported, the default
is `localhost:9154`. The metrics path is fixed to `/metrics`.

## Examples
