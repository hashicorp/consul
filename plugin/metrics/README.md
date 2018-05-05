# prometheus

## Name

*prometheus* - enables [Prometheus](https://prometheus.io/) metrics.

## Description

With *prometheus* you export metrics from CoreDNS and any plugin that has them.
The default location for the metrics is `localhost:9153`. The metrics path is fixed to `/metrics`.
The following metrics are exported:

* `coredns_build_info{version, revision, goversion}` - info about CoreDNS itself.
* `coredns_panic_count_total{}` - total number of panics.
* `coredns_dns_request_count_total{server, zone, proto, family}` - total query count.
* `coredns_dns_request_duration_seconds{server, zone}` - duration to process each query.
* `coredns_dns_request_size_bytes{server, zone, proto}` - size of the request in bytes.
* `coredns_dns_request_do_count_total{server, zone}` -  queries that have the DO bit set
* `coredns_dns_request_type_count_total{server, zone, type}` - counter of queries per zone and type.
* `coredns_dns_response_size_bytes{server, zone, proto}` - response size in bytes.
* `coredns_dns_response_rcode_count_total{server, zone, rcode}` - response per zone and rcode.

Each counter has a label `zone` which is the zonename used for the request/response.

Extra labels used are:

* `server` is identifying the server responsible for the request. This is a string formatted
  as the server's listening address: `<scheme>://[<bind>]:<port>`. I.e. for a "normal" DNS server
  this is `dns://:53`. If you are using the *bind* plugin an IP address is included, e.g.: `dns://127.0.0.53:53`.
* `proto` which holds the transport of the response ("udp" or "tcp")
* The address family (`family`) of the transport (1 = IP (IP version 4), 2 = IP6 (IP version 6)).
* `type` which holds the query type. It holds most common types (A, AAAA, MX, SOA, CNAME, PTR, TXT,
  NS, SRV, DS, DNSKEY, RRSIG, NSEC, NSEC3, IXFR, AXFR and ANY) and "other" which lumps together all
  other types.
* The `response_rcode_count_total` has an extra label `rcode` which holds the rcode of the response.

If monitoring is enabled, queries that do not enter the plugin chain are exported under the fake
name "dropped" (without a closing dot - this is never a valid domain name).

This plugin can only be used once per Server Block.

## Syntax

~~~
prometheus [ADDRESS]
~~~

For each zone that you want to see metrics for.

It optionally takes an address to which the metrics are exported; the default
is `localhost:9153`. The metrics path is fixed to `/metrics`.

## Examples

Use an alternative address:

~~~ corefile
. {
    prometheus localhost:9253
}
~~~

Or via an enviroment variable (this is supported throughout the Corefile): `export PORT=9253`, and
then:

~~~ corefile
. {
    prometheus localhost:{$PORT}
}
~~~

## Bugs

When reloading, we keep the handler running, meaning that any changes to the handler's address
aren't picked up. You'll need to restart CoreDNS for that to happen.
