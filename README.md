# CoreDNS

CoreDNS is DNS server that started as a fork of [Caddy](https://github.com/mholt/caddy/). It has the
same model: it chains middleware.

## Status

Currently CoreDNS is able to:

* Serve zone data from a file, both DNSSEC (NSEC only atm) and DNS is supported.
* Retrieve zone data from primaries, i.e. act as a secondary server.
* Allow for zone transfers, i.e. act as a primary server.
* Use Etcd as a backend, i.e. a 90% replacement for
  [SkyDNS](https://github.com/skynetservices/skydns).
* Serve as a proxy to forward queries to some other (recursive) nameserver.
* Rewrite queries (both qtype, qclass and qname).
* Provide metrics (by using Prometheus)
* Provide Logging.

There are corner cases not implemented and a few [issues](https://github.com/miekg/coredns/issues).

But all in all, CoreDNS should already be able to provide you with enough functionality to replace
parts of BIND9, Knot, NSD or PowerDNS.
However CoreDNS is still in the early stages of development and should **not** be used on production
servers yet. For now most documentation is in the source and some blog articles can be [found
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
* Code simplifications/refactors.
* Optimizations.
* Load testing.
* All the [issues](https://github.com/miekg/coredns/issues).

## Blog

<https://miek.nl/tags/coredns/>
