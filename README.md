# CoreDNS

CoreDNS is a DNS server that started as a fork of [Caddy](https://github.com/mholt/caddy/). It has the
same model: it chains middleware. In fact it's so similar that CoreDNS is now a server type plugin for
Caddy, i.e., you'll need Caddy to compile CoreDNS.

CoreDNS is the successor to [SkyDNS](https://github.com/skynetservices/skydns). SkyDNS is a thin
layer that exposes services in etcd in the DNS. CoreDNS builds on this idea and is a generic DNS
server that can talk to multiple backends (etcd, consul, kubernetes, etc.).

CoreDNS aims to be a fast and flexible DNS server. The keyword here is *flexible*: with CoreDNS you
are able to do what you want with your DNS data. And if not: write some middleware!

Currently CoreDNS is able to:

* Serve zone data from a file; both DNSSEC (NSEC only) and DNS are supported (middleware/file).
* Retrieve zone data from primaries, i.e., act as a secondary server (AXFR only) (middleware/secondary).
* Sign zone data on-the-fly (middleware/dnssec).
* Load balancing of responses (middleware/loadbalance).
* Allow for zone transfers, i.e., act as a primary server (middleware/file).
* Caching (middleware/cache).
* Health checking (middleware/health).
* Use etcd as a backend, i.e., a 101.5% replacement for
  [SkyDNS](https://github.com/skynetservices/skydns) (middleware/etcd).
* Use k8s (kubernetes) as a backend (middleware/kubernetes).
* Serve as a proxy to forward queries to some other (recursive) nameserver (middleware/proxy).
* Rewrite queries (qtype, qclass and qname) (middleware/rewrite).
* Provide metrics (by using Prometheus) (middleware/metrics).
* Provide Logging (middleware/log).
* Support the CH class: `version.bind` and friends (middleware/chaos).
* Profiling support (middleware/pprof).

Each of the middlewares has a README.md of its own.

## Status

I'm using CoreDNS is my primary, authoritative, nameserver for my domains (`miek.nl`, `atoom.net`
and a few others). CoreDNS should be stable enough to provide you with good DNS(SEC) service.

There are still few [issues](https://github.com/miekg/coredns/issues), and work is ongoing on making
things fast and to reduce the memory usage.

All in all, CoreDNS should be able to provide you with enough functionality to replace parts of
BIND 9, Knot, NSD or PowerDNS and SkyDNS.
Most documentation is in the source and some blog articles can be [found
here](https://miek.nl/tags/coredns/). If you do want to use CoreDNS in production, please let us
know and how we can help.

<https://caddyserver.com/> is also full of examples on how to structure a Corefile (renamed from
Caddyfile when I forked it).

## Compilation

CoreDNS (as a servertype plugin for Caddy) has a dependency on Caddy - this is *almost* like
the normal Go dependencies, but with a small twist: caddy (the source) need to know that CoreDNS
exists and for this we need to add 1 line `_ "github.com/miekg/coredns/core"` to file in caddy.

You have the source of CoreDNS, this should preferably be downloaded under your `$GOPATH`. Get all
dependencies:

    go get ./...

Then, execute `go generate`. This will patch Caddy to add CoreDNS (and remove the HTTP server
plugin), and then `go build` as you would normally do:

    go generate
    go build

This should yield a `coredns` binary.

## Examples

Start a simple proxy, you'll need to be root to start listening on port 53.

`Corefile` contains:

~~~ txt
.:53 {
    proxy . 8.8.8.8:53
    log stdout
}
~~~

Just start CoreDNS: `./coredns`.
And then just query on that port (53). The query should be forwarded to 8.8.8.8 and the response
will be returned. Each query should also show up in the log.

Serve the (NSEC) DNSSEC-signed `example.org` on port 1053, with errors and logging sent to stdout. Allow zone
transfers to everybody, but specically mention 1 IP address so that CoreDNS can send notifies to it.

~~~ txt
example.org:1053 {
    file /var/lib/coredns/example.org.signed {
        transfer to *
        transfer to 2001:500:8f::53
    }
    errors stdout
    log stdout
}
~~~

Serve `example.org` on port 1053, but forward everything that does *not* match `example.org` to a recursive
nameserver *and* rewrite ANY queries to HINFO.

~~~ txt
.:1053 {
    rewrite ANY HINFO
    proxy . 8.8.8.8:53

    file /var/lib/coredns/example.org.signed example.org {
        transfer to *
        transfer to 2001:500:8f::53
    }
    errors stdout
    log stdout
}
~~~


## What Remains To Be Done

* Optimizations.
* Load testing.
* The [issues](https://github.com/miekg/coredns/issues).


## Blog and Contact

Website: <https://coredns.io>
Twitter: `@coredns.io`
Docs: <https://miek.nl/tags/coredns/>
Github: <https://github.com/miekg/coredns>


## Systemd Service File

Use this as a systemd service file. It defaults to a coredns wich a homedir of /home/coredns
and the binary lives in /opt/bin and the config in `/etc/coredns/Corefile`:

~~~ txt
[Unit]
Description=CoreDNS DNS server
Documentation=https://coredns.io
After=network.target

[Service]
PermissionsStartOnly=true
PIDFile=/home/coredns/coredns.pid
LimitNOFILE=8192
User=coredns
WorkingDirectory=/home/coredns
ExecStartPre=/sbin/setcap cap_net_bind_service=+ep /opt/bin/coredns
ExecStart=/opt/bin/coredns -pidfile /home/coredns/coredns.pid -conf=/etc/coredns/Corefile
ExecReload=/bin/kill -SIGUSR1 $MAINPID
Restart=on-failure

[Install]
WantedBy=multi-user.target
~~~
