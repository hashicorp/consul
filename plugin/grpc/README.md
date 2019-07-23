# grpc

## Name

*grpc* - facilitates proxying DNS messages to upstream resolvers via gRPC protocol.

## Description

The *grpc* plugin supports gRPC and TLS.

This plugin can only be used once per Server Block.

## Syntax

In its most basic form:

~~~
grpc FROM TO...
~~~

* **FROM** is the base domain to match for the request to be proxied.
* **TO...** are the destination endpoints to proxy to. The number of upstreams is
  limited to 15.

Multiple upstreams are randomized (see `policy`) on first use. When a proxy returns an error
the next upstream in the list is tried.

Extra knobs are available with an expanded syntax:

~~~
grpc FROM TO... {
    except IGNORED_NAMES...
    tls CERT KEY CA
    tls_servername NAME
    policy random|round_robin|sequential
}
~~~

* **FROM** and **TO...** as above.
* **IGNORED_NAMES** in `except` is a space-separated list of domains to exclude from proxying.
  Requests that match none of these names will be passed through.
* `tls` **CERT** **KEY** **CA** define the TLS properties for TLS connection. From 0 to 3 arguments can be
  provided with the meaning as described below

  * `tls` - no client authentication is used, and the system CAs are used to verify the server certificate
  * `tls` **CA** - no client authentication is used, and the file CA is used to verify the server certificate
  * `tls` **CERT** **KEY** - client authentication is used with the specified cert/key pair.
    The server certificate is verified with the system CAs
  * `tls` **CERT** **KEY**  **CA** - client authentication is used with the specified cert/key pair.
    The server certificate is verified using the specified CA file

* `tls_servername` **NAME** allows you to set a server name in the TLS configuration; for instance 9.9.9.9
  needs this to be set to `dns.quad9.net`. Multiple upstreams are still allowed in this scenario,
  but they have to use the same `tls_servername`. E.g. mixing 9.9.9.9 (QuadDNS) with 1.1.1.1
  (Cloudflare) will not work.
* `policy` specifies the policy to use for selecting upstream servers. The default is `random`.

Also note the TLS config is "global" for the whole grpc proxy if you need a different
`tls-name` for different upstreams you're out of luck.

## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metric are exported:

* `coredns_grpc_request_duration_seconds{to}` - duration per upstream interaction.
* `coredns_grpc_request_count_total{to}` - query count per upstream.
* `coredns_grpc_response_rcode_count_total{to, rcode}` - count of RCODEs per upstream.
  and we are randomly (this always uses the `random` policy) spraying to an upstream.

## Examples

Proxy all requests within `example.org.` to a nameserver running on a different port:

~~~ corefile
example.org {
    grpc . 127.0.0.1:9005
}
~~~

Load balance all requests between three resolvers, one of which has a IPv6 address.

~~~ corefile
. {
    grpc . 10.0.0.10:53 10.0.0.11:1053 [2003::1]:53
}
~~~

Forward everything except requests to `example.org`

~~~ corefile
. {
    grpc . 10.0.0.10:1234 {
        except example.org
    }
}
~~~

Proxy everything except `example.org` using the host's `resolv.conf`'s nameservers:

~~~ corefile
. {
    grpc . /etc/resolv.conf {
        except example.org
    }
}
~~~

Proxy all requests to 9.9.9.9 using the TLS protocol, and cache every answer for up to 30
seconds. Note the `tls_servername` is mandatory if you want a working setup, as 9.9.9.9 can't be
used in the TLS negotiation.

~~~ corefile
. {
    grpc . 9.9.9.9 {
       tls_servername dns.quad9.net
    }
    cache 30
}
~~~

Or with multiple upstreams from the same provider

~~~ corefile
. {
    grpc . 1.1.1.1 1.0.0.1 {
       tls_servername cloudflare-dns.com
    }
    cache 30
}
~~~

## Bugs

The TLS config is global for the whole grpc proxy if you need a different `tls_servername` for
different upstreams you're out of luck.
