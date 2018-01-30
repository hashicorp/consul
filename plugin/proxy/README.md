# proxy

## Name

*proxy* - facilitates both a basic reverse proxy and a robust load balancer.

## Description

The proxy has support for multiple backends. The load balancing features include multiple policies,
health checks, and failovers. If all hosts fail their health check the proxy plugin will fail
back to randomly selecting a target and sending packets to it.

## Syntax

In its most basic form, a simple reverse proxy uses this syntax:

~~~
proxy FROM TO
~~~

* **FROM** is the base domain to match for the request to be proxied.
* **TO** is the destination endpoint to proxy to.

However, advanced features including load balancing can be utilized with an expanded syntax:

~~~
proxy FROM TO... {
    policy random|least_conn|round_robin|first
    fail_timeout DURATION
    max_fails INTEGER
    health_check PATH:PORT [DURATION]
    except IGNORED_NAMES...
    spray
    protocol [dns [force_tcp]|https_google [bootstrap ADDRESS...]|grpc [insecure|CACERT|KEY CERT|KEY CERT CACERT]]
}
~~~

* **FROM** is the name to match for the request to be proxied.
* **TO** is the destination endpoint to proxy to. At least one is required, but multiple may be
  specified. **TO** may be an IP:Port pair, or may reference a file in resolv.conf format
* `policy` is the load balancing policy to use; applies only with multiple backends. May be one of
  random, least_conn, round_robin or first. Default is random.
* `fail_timeout` specifies how long to consider a backend as down after it has failed. While it is
  down, requests will not be routed to that backend. A backend is "down" if CoreDNS fails to
  communicate with it. The default value is 2 seconds ("2s").
* `max_fails` is the number of failures within fail_timeout that are needed before considering
  a backend to be down. If 0, the backend will never be marked as down. Default is 1.
* `health_check` will check **PATH** (on **PORT**) on each backend. If a backend returns a status code of
  200-399, then that backend is marked healthy for double the healthcheck duration.  If it doesn't,
  it is marked as unhealthy and no requests are routed to it.  If this option is not provided then
  health checks are disabled.  The default duration is 4 seconds ("4s").
* **IGNORED_NAMES** in `except` is a space-separated list of domains to exclude from proxying.
  Requests that match none of these names will be passed through.
* `spray` when all backends are unhealthy, randomly pick one to send the traffic to. (This is
  a failsafe.)
* `protocol` specifies what protocol to use to speak to an upstream, `dns` (the default) is plain
  old DNS, and `https_google` uses `https://dns.google.com` and speaks a JSON DNS dialect. Note when
  using this **TO** will be ignored. The `grpc` option will talk to a server that has implemented
  the [DnsService](https://github.com/coredns/coredns/pb/dns.proto).
  An out-of-tree plugin that implements the server side of this can be found at
  [here](https://github.com/infobloxopen/coredns-grpc).

## Policies

There are three load-balancing policies available:
* `random` (default) - Randomly select a backend
* `least_conn` - Select the backend with the fewest active connections
* `round_robin` - Select the backend in round-robin fashion

All polices implement randomly spraying packets to backend hosts when *no healthy* hosts are
available. This is to preeempt the case where the healthchecking (as a mechanism) fails.

## Upstream Protocols

Currently `protocol` supports `dns` (i.e., standard DNS over UDP/TCP) and `https_google` (JSON
payload over HTTPS). Note that with `https_google` the entire transport is encrypted. Only *you* and
*Google* can see your DNS activity.

`dns`
:   uses the standard DNS exchange. You can pass `force_tcp` to make sure that the proxied connection is performed
    over TCP, regardless of the inbound request's protocol.

`grpc`
:   extra options are used to control how the TLS connection is made to the gRPC server.

  * None - No client authentication is used, and the system CAs are used to verify the server certificate.
  * `insecure` - TLS is not used, the connection is made in plaintext (not good in production).
  * **CACERT** - No client authentication is used, and the file **CACERT** is used to verify the server certificate.
  * **KEY** **CERT** - Client authentication is used with the specified key/cert pair. The server
     certificate is verified with the system CAs.
  * **KEY** **CERT** **CACERT** - Client authentication is used with the specified key/cert pair. The
     server certificate is verified using the **CACERT** file.
  An out-of-tree plugin that implements the server side of this can be found at
  [here](https://github.com/infobloxopen/coredns-grpc).

`https_google`
:    bootstrap **ADDRESS...** is used to (re-)resolve `dns.google.com`.

    This happens every 300s. If not specified the default is used: 8.8.8.8:53/8.8.4.4:53.
    Note that **TO** is *ignored* when `https_google` is used, as its upstream is defined as `dns.google.com`.


## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metric is exported:

* `coredns_proxy_request_duration_seconds{proto, proto_proxy, family, to}` - duration per upstream
  interaction.
* `coredns_proxy_request_count_total{proto, proto_proxy, family, to}` - query count per upstream.

Where `proxy_proto` is the protocol used (`dns`, `grpc`, or `https_google`) and `to` is **TO**
specified in the config, `proto` is the protocol used by the incoming query ("tcp" or "udp").
and family the transport family ("1" for IPv4, and "2" for IPv6).

## Examples

Proxy all requests within example.org. to a backend system:

~~~
proxy example.org 127.0.0.1:9005
~~~

Load-balance all requests between three backends (using random policy):

~~~ corefile
. {
    proxy . 10.0.0.10:53 10.0.0.11:1053 10.0.0.12
}
~~~

Same as above, but round-robin style:

~~~ corefile
. {
    proxy . 10.0.0.10:53 10.0.0.11:1053 10.0.0.12 {
        policy round_robin
    }
}
~~~

With health checks and proxy headers to pass hostname, IP, and scheme upstream:

~~~ corefile
. {
    proxy . 10.0.0.11:53 10.0.0.11:53 10.0.0.12:53 {
        policy round_robin
        health_check /health:8080
    }
}
~~~

Proxy everything except requests to miek.nl or example.org

~~~
. {
    proxy . 10.0.0.10:1234 {
        except miek.nl example.org
    }
}
~~~

Proxy everything except `example.org` using the host's `resolv.conf`'s nameservers:

~~~ corefile
. {
    proxy . /etc/resolv.conf {
        except miek.nl example.org
    }
}
~~~

Proxy all requests within `example.org` to Google's `dns.google.com`.

~~~ corefile
. {
    proxy example.org 1.2.3.4:53 {
        protocol https_google
    }
}
~~~

Proxy everything with HTTPS to `dns.google.com`, except `example.org`. Then have another proxy in
another stanza that uses plain DNS to resolve names under `example.org`.

~~~ corefile
. {
    proxy . 1.2.3.4:53 {
        except example.org
        protocol https_google
    }
}

example.org {
    proxy . 8.8.8.8:53
}
~~~

## Bugs

When using the `google_https` protocol the health checking will health check the wrong endpoint.
See <https://github.com/coredns/coredns/issues/1202> for some background.
