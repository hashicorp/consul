# CoreDNS

CoreDNS is DNS server that started as a fork of [Caddy](https://github.com/mholt/caddy/). It has the
same model: it chains middleware.

Currently CoreDNS is able to:

* Serve zone data from a file, both DNSSEC (NSEC only) and DNS is supported (middleware/file).
* Retrieve zone data from primaries, i.e. act as a secondary server (middleware/file).
* Loadbalancing of responses (middleware/loadbalance).
* Allow for zone transfers, i.e. act as a primary server (middleware/file).
* Caching (middleware/cache).
* Use etcd as a backend, i.e. a 98.5% replacement for
  [SkyDNS](https://github.com/skynetservices/skydns) (middleware/etcd).
* Serve as a proxy to forward queries to some other (recursive) nameserver (middleware/proxy).
* Rewrite queries (both qtype, qclass and qname) (middleware/rewrite).
* Provide metrics (by using Prometheus) (middleware/metrics).
* Provide Logging (middleware/log).
* Has support for the CH class: `version.bind` and friends (middleware/chaos).

## Status

I'm using CoreDNS is my primary, authoritative, nameserver for my domains (`miek.nl`, `atoom.net`
and a few others). CoreDNS should be stable enough to provide you with a good DNS(SEC) service.

There are still few [issues](https://github.com/miekg/coredns/issues), and work is ongoing on making
things fast and reduce the memory usage.

All in all, CoreDNS should be able to provide you with enough functionality to replace parts of
BIND9, Knot, NSD or PowerDNS.
Most documentation is in the source and some blog articles can be [found
here](https://miek.nl/tags/coredns/). If you do want to use CoreDNS in production, please let us
know and how we can help.

<https://caddyserver.com/> is also full of examples on how to structure a Corefile (renamed from
Caddyfile when I forked it).

## Examples

Start a simple proxy:

`Corefile` contains:

~~~ txt
.:1053 {
    proxy . 8.8.8.8:53
}
~~~

Just start CoreDNS: `./coredns`.
And then just query on that port (1053), the query should be forwarded to 8.8.8.8 and the response
will be returned.

Serve the (NSEC) DNSSEC signed `miek.nl` on port 1053, errors and logging to stdout. Allow zone
transfers to everybody.

~~~ txt
miek.nl:1053 {
    file /var/lib/bind/miek.nl.signed {
        transfer to *
    }
    errors stdout
    log stdout
}
~~~

Serve `miek.nl` on port 1053, but forward everything that does *not* match `miek.nl` to a recursive
nameserver *and* rewrite ANY queries to HINFO.

~~~ txt
.:1053 {
    rewrite ANY HINFO
    proxy . 8.8.8.8:53

    file /var/lib/bind/miek.nl.signed miek.nl {
        transfer to *
    }
    errors stdout
    log stdout
}
~~~

All the above examples are possible with the *current* CoreDNS.

## What remains to be done

* Website?
* Logo?
* Optimizations.
* Load testing.
* The [issues](https://github.com/miekg/coredns/issues).

## Blog

<https://miek.nl/tags/coredns/>
