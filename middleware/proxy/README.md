# proxy

`proxy` facilitates both a basic reverse proxy and a robust load balancer. The proxy has support for
multiple backends and adding custom headers. The load balancing features include multiple policies,
health checks, and failovers.

## Syntax

In its most basic form, a simple reverse proxy uses this syntax:

~~~
proxy from to
~~~

* `from` is the base path to match for the request to be proxied
* `to` is the destination endpoint to proxy to

However, advanced features including load balancing can be utilized with an expanded syntax:

~~~
proxy from to... {
    policy random | least_conn | round_robin
    fail_timeout duration
    max_fails integer
    health_check path:port [duration]
    except ignored_names...
    ecs [v4 address/mask] [v6 address/mask] (TODO)
}
~~~

* `from` is the base path to match for the request to be proxied.
* `to` is the destination endpoint to proxy to. At least one is required, but multiple may be specified.
* `policy` is the load balancing policy to use; applies only with multiple backends. May be one of random, least_conn, or round_robin. Default is random.
* `fail_timeout` specifies how long to consider a backend as down after it has failed. While it is down, requests will not be routed to that backend. A backend is "down" if CoreDNS fails to communicate with it. The default value is 10 seconds ("10s").
* `max_fails` is the number of failures within fail_timeout that are needed before considering a backend to be down. If 0, the backend will never be marked as down. Default is 1.
* `health_check` will check path (on port) on each backend. If a backend returns a status code of 200-399, then that backend is healthy. If it doesn't, the backend is marked as unhealthy for duration and no requests are routed to it. If this option is not provided then health checks are disabled. The default duration is 10 seconds ("10s").
* `ignored_names...` is a space-separated list of paths to exclude from proxying. Requests that match any of these paths will be passed thru.
* `ecs` add EDNS0 client submit metadata to the outgoing query. This can be optionally be followed
  by an IPv4 and/or IPv6 address. If none is specified the server's addresses are used.

## Policies

There are three load balancing policies available:
* *random* (default) - Randomly select a backend
* *least_conn* - Select backend with the fewest active connections
* *round_robin* - Select backend in round-robin fashion

## Examples

Proxy all requests within example.org. to a backend system:

~~~
proxy example.org localhost:9005
~~~

Load-balance all requests between three backends (using random policy):

~~~
proxy . web1.local:53 web2.local:1053 web3.local
~~~

Same as above, but round-robin style:

~~~
proxy . web1.local:53 web2.local:1053 web3.local {
	policy round_robin
}
~~~

With health checks and proxy headers to pass hostname, IP, and scheme upstream:

~~~
proxy . web1.local:53 web2.local:53 web3.local:53 {
	policy round_robin
	health_check /health:8080
}
~~~

Proxy everything except requests to miek.nl or example.org

~~~
proxy . backend:1234 {
	except miek.nl example.org
}
~~~
