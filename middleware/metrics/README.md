# prometheus

This module enables prometheus metrics for CoreDNS. The default location for the metrics is
`localhost:9153`. The metrics path is fixed to `/metrics`.

The following metrics are exported:

* coredns_dns_request_count_total{zone, proto, family}
* coredns_dns_request_duration_milliseconds{zone}
* coredns_dns_request_size_bytes{zone, proto}
* coredns_dns_request_do_count_total{zone}
* coredns_dns_request_type_count_total{zone, type}
* coredns_dns_response_size_bytes{zone, proto}
* coredns_dns_response_rcode_count_total{zone, rcode}

Each counter has a label `zone` which is the zonename used for the request/response.

Extra labels used are:

* `proto` which holds the transport of the response ("udp" or "tcp")
* The address family (`family`) of the transport (1 = IP (IP version 4), 2 = IP6 (IP version 6)).
* `type` which holds the query type. It holds most common types (A, AAAA, MX, SOA, CNAME, PTR, TXT,
  NS, SRV, DS, DNSKEY, RRSIG, NSEC, NSEC3, IXFR, AXFR and ANY) and "other" which lumps together all
  other types.
* The `response_rcode_count_total` has an extra label `rcode` which holds the rcode of the response.

If monitoring is enabled, queries that do not enter the middleware chain are exported under the fake
name "dropped" (without a closing dot - this is never a valid domain name).

## Syntax

~~~
prometheus [ADDRESS]
~~~

For each zone that you want to see metrics for.

It optionally takes an address to which the metrics are exported; the default
is `localhost:9153`. The metrics path is fixed to `/metrics`.

## Examples

Use an alternative address:

~~~
prometheus localhost:9253
~~~
