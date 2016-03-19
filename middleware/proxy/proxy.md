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
	health_check path [duration]
	proxy_header name value
	without prefix
	except ignored_paths...
	insecure_skip_verify
	preset
}
~~~

* from is the base path to match for the request to be proxied.
* to is the destination endpoint to proxy to. At least one is required, but multiple may be specified.
* policy is the load balancing policy to use; applies only with multiple backends. May be one of random, least_conn, or round_robin. Default is random.
* fail_timeout specifies how long to consider a backend as down after it has failed. While it is down, requests will not be routed to that backend. A backend is "down" if Caddy fails to communicate with it. The default value is 10 seconds ("10s").
* max_fails is the number of failures within fail_timeout that are needed before considering a backend to be down. If 0, the backend will never be marked as down. Default is 1.
* health_check will check path on each backend. If a backend returns a status code of 200-399, then that backend is healthy. If it doesn't, the backend is marked as unhealthy for duration and no requests are routed to it. If this option is not provided then health checks are disabled. The default duration is 10 seconds ("10s").
* proxy_header sets headers to be passed to the backend. The field name is name and the value is value. This option can be specified multiple times for multiple headers, and dynamic values can also be inserted using request placeholders.
* prefix is a URL prefix to trim before proxying the request upstream. A request to /api/foo without /api, for example, will result in a proxy request to /foo.
* ignored_paths... is a space-separated list of paths to exclude from proxying. Requests that match any of these paths will be passed thru.
* insecure_skip_verify overrides verification of the backend TLS certificate, essentially disabling security features over HTTPS.

## Policies

There are three load balancing policies available:
* random (default) - Randomly select a backend
* least_conn - Select backend with the fewest active connections
* round_robin - Select backend in round-robin fashion

## Examples

Proxy all requests within /api to a backend system:
proxy /api localhost:9005
Load-balance all requests between three backends (using random policy):
proxy / web1.local:80 web2.local:90 web3.local:100

Same as above, but round-robin style:

proxy / web1.local:80 web2.local:90 web3.local:100 {
	policy round_robin
}

With health checks and proxy headers to pass hostname, IP, and scheme upstream:

proxy / web1.local:80 web2.local:90 web3.local:100 {
	policy round_robin
	health_check /health
	proxy_header Host {host}
	proxy_header X-Real-IP {remote}
	proxy_header X-Forwarded-Proto {scheme}
}
Proxy WebSocket connections:
proxy / localhost:8080 {
	websocket
}
Proxy everything except requests to /static or /robots.txt:
proxy / backend:1234 {
	except /static /robots.txt
}
