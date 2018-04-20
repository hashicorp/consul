# forward

## Name

*forward* - facilitates proxying DNS messages to upstream resolvers.

## Description

The *forward* plugin re-uses already opened sockets to the upstreams. It supports UDP, TCP and
DNS-over-TLS and uses in band health checking.

When it detects an error a health check is performed. This checks runs in a loop, every *0.5s*, for
as long as the upstream reports unhealthy. Once healthy we stop health checking (until the next
error). The health checks use a recursive DNS query (`. IN NS`) to get upstream health. Any response
that is not a network error (REFUSED, NOTIMPL, SERVFAIL, etc) is taken as a healthy upstream. The
health check uses the same protocol as specified in **TO**. If `max_fails` is set to 0, no checking
is performed and upstreams will always be considered healthy.

When *all* upstreams are down it assumes health checking as a mechanism has failed and will try to
connect to a random upstream (which may or may not work).

This plugin can only be used once per Server Block.

## Syntax

In its most basic form, a simple forwarder uses this syntax:

~~~
forward FROM TO...
~~~

* **FROM** is the base domain to match for the request to be forwarded.
* **TO...** are the destination endpoints to forward to. The **TO** syntax allows you to specify
  a protocol, `tls://9.9.9.9` or `dns://` (or no protocol) for plain DNS. The number of upstreams is
  limited to 15.

Multiple upstreams are randomized (see `policy`) on first use. When a healthy proxy returns an error
during the exchange the next upstream in the list is tried.

Extra knobs are available with an expanded syntax:

~~~
forward FROM TO... {
    except IGNORED_NAMES...
    force_tcp
    expire DURATION
    max_fails INTEGER
    tls CERT KEY CA
    tls_servername NAME
    policy random|round_robin|sequential
    health_check DURATION
}
~~~

* **FROM** and **TO...** as above.
* **IGNORED_NAMES** in `except` is a space-separated list of domains to exclude from forwarding.
  Requests that match none of these names will be passed through.
* `force_tcp`, use TCP even when the request comes in over UDP.
* `max_fails` is the number of subsequent failed health checks that are needed before considering
  an upstream to be down. If 0, the upstream will never be marked as down (nor health checked).
  Default is 2.
* `expire` **DURATION**, expire (cached) connections after this time, the default is 10s.
* `tls` **CERT** **KEY** **CA** define the TLS properties for TLS connection. From 0 to 3 arguments can be
  provided with the meaning as described below
  * `tls` - no client authentication is used, and the system CAs are used to verify the server certificate
  * `tls` **CA** - no client authentication is used, and the file CA is used to verify the server certificate
  * `tls` **CERT** **KEY** - client authentication is used with the specified cert/key pair.
    The server certificate is verified with the system CAs
  * `tls` **CERT** **KEY**  **CA** - client authentication is used with the specified cert/key pair.
    The server certificate is verified using the specified CA file
* `tls_servername` **NAME** allows you to set a server name in the TLS configuration; for instance 9.9.9.9
  needs this to be set to `dns.quad9.net`.
* `policy` specifies the policy to use for selecting upstream servers. The default is `random`.
* `health_check`, use a different **DURATION** for health checking, the default duration is 0.5s.

Also note the TLS config is "global" for the whole forwarding proxy if you need a different
`tls-name` for different upstreams you're out of luck.

## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metric are exported:

* `coredns_forward_request_duration_seconds{to}` - duration per upstream interaction.
* `coredns_forward_request_count_total{to}` - query count per upstream.
* `coredns_forward_response_rcode_total{to, rcode}` - count of RCODEs per upstream.
* `coredns_forward_healthcheck_failure_count_total{to}` - number of failed health checks per upstream.
* `coredns_forward_healthcheck_broken_count_total{}` - counter of when all upstreams are unhealthy,
  and we are randomly (this always uses the `random` policy) spraying to an upstream.
* `coredns_forward_socket_count_total{to}` - number of cached sockets per upstream.

Where `to` is one of the upstream servers (**TO** from the config), `proto` is the protocol used by
the incoming query ("tcp" or "udp"), and family the transport family ("1" for IPv4, and "2" for
IPv6).

## Examples

Proxy all requests within example.org. to a nameserver running on a different port:

~~~ corefile
example.org {
    forward . 127.0.0.1:9005
}
~~~

Load balance all requests between three resolvers, one of which has a IPv6 address.

~~~ corefile
. {
    forward . 10.0.0.10:53 10.0.0.11:1053 [2003::1]:53
}
~~~

Forward everything except requests to `example.org`

~~~ corefile
. {
    forward . 10.0.0.10:1234 {
        except example.org
    }
}
~~~

Proxy everything except `example.org` using the host's `resolv.conf`'s nameservers:

~~~ corefile
. {
    forward . /etc/resolv.conf {
        except example.org
    }
}
~~~

Proxy all requests to 9.9.9.9 using the DNS-over-TLS protocol, and cache every answer for up to 30
seconds. Note the `tls_servername` is mandatory if you want a working setup, as 9.9.9.9 can't be
used in the TLS negotiation. Also set the health check duration to 5s to not completely swamp the
service with health checks.

~~~ corefile
. {
    forward . tls://9.9.9.9 {
       tls_servername dns.quad9.net
       health_check 5s
    }
    cache 30
}
~~~

## Bugs

The TLS config is global for the whole forwarding proxy if you need a different `tls_serveraame` for
different upstreams you're out of luck.

## Also See

[RFC 7858](https://tools.ietf.org/html/rfc7858) for DNS over TLS.
