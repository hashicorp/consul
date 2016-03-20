# CoreDNS

CoreDNS is DNS server that started as a fork of [Caddy](https://github.com/mholt/caddy/). It has the
same model: it chains middleware.

It is in the early stages of development and should **not** be used on production servers yet. For now most
documentation is in the source and some blog articles can be [found
here](https://miek.nl/tags/coredns/).

<https://caddyserver.com/> is also full of examples on how to structure a Corefile (renamed from
Caddyfile when I forked it).

# Resolver

Start a simple resolver (proxy):

`Corefile` contains:

~~~
.:1053 {
    proxy . 8.8.8.8:53
}
~~~

Just start CoreDNS: `./coredns`.
And then just query on that port (1053), the query should be forwarded to 8.8.8.8 and the response
will be returned.

# Blog

<https://miek.nl/tags/coredns/>
