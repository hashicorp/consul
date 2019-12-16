# consul

## Name

*consul* - enables SkyDNS service discovery from consul.

## Description

The *consul* plugin implements the (older) SkyDNS service discovery service. It is *not* suitable as
a generic DNS zone data plugin. Only a subset of DNS record types are implemented, and subdomains
and delegations are not handled at all.

The data in the consul instance has to be encoded as
a [message](https://github.com/skynetservices/skydns/blob/2fcff74cdc9f9a7dd64189a447ef27ac354b725f/msg/service.go#L26)
like [SkyDNS](https://github.com/skynetservices/skydns). It works just like SkyDNS.

The consul plugin makes extensive use of the *forward* plugin to forward and query other servers in the
network.

## Syntax

~~~
consul [ZONES...]
~~~

* **ZONES** zones *consul* should be authoritative for.

The path will default to `/skydns` the local consul proxy (http://localhost:8500). If no zones are
specified the block's zone will be used as the zone.

If you want to `round robin` A and AAAA responses look at the `loadbalance` plugin.

~~~
consul [ZONES...] {
    fallthrough [ZONES...]
    path PATH
    address address
    token token
}
~~~

* `fallthrough` If zone matches but no record can be generated, pass request to the next plugin.
  If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.
* **PATH** the path inside consul. Defaults to "/skydns".
* **ADDRESS** the consul endpoints. Defaults to "http://localhost:8500".


## Special Behaviour

The *consul* plugin leverages directory structure to look for related entries. For example
an entry `/skydns/test/skydns/mx` would have entries like `/skydns/test/skydns/mx/a`,
`/skydns/test/skydns/mx/b` and so on. Similarly a directory `/skydns/test/skydns/mx1` will have all
`mx1` entries.

## Examples

This is the default SkyDNS setup, with everything specified in full:

~~~ corefile
skydns.local {
    consul {
        path /skydns
        address http://localhost:8500
        token xxxx-xxxx-xxxx-xxxx-xxxx
    }
    prometheus
    cache
    loadbalance
}

. {
    forward . 8.8.8.8:53 8.8.4.4:53
    cache
}
~~~

Or a setup where we use `/etc/resolv.conf` as the basis for the proxy and the upstream
when resolving external pointing CNAMEs.

~~~ corefile
skydns.local {
    consul {
        path /skydns
    }
    cache
}

. {
    forward . /etc/resolv.conf
    cache
}
~~~


### Reverse zones

Reverse zones are supported. You need to make CoreDNS aware of the fact that you are also
authoritative for the reverse. For instance if you want to add the reverse for 10.0.0.0/24, you'll
need to add the zone `0.0.10.in-addr.arpa` to the list of zones. Showing a snippet of a Corefile:

~~~
consul skydns.local 10.0.0.0/24 {
...
~~~

Next you'll need to populate the zone with reverse records, here we add a reverse for
10.0.0.127 pointing to reverse.skydns.local.

~~~
% /skydns/arpa/in-addr/10/0/0/127 '{"host":"reverse.skydns.local."}'
~~~

Querying with dig:

~~~ sh
% dig @localhost -x 10.0.0.127 +short
reverse.skydns.local.
~~~

### Zone name as A record

The zone name itself can be used as an `A` record. This behavior can be achieved by writing special
entries to the ETCD path of your zone. If your zone is named `skydns.local` for example, you can
create an `A` record for this zone as follows:

~~~
% /skydns/local/skydns/ '{"host":"1.1.1.1","ttl":60}'
~~~

If you query the zone name itself, you will receive the created `A` record:

~~~ sh
% dig +short skydns.local @localhost
1.1.1.1
~~~

If you would like to use DNS RR for the zone name, you can set the following:
~~~
% /skydns/local/skydns/x1 '{"host":"1.1.1.1","ttl":60}'
% /skydns/local/skydns/x2 '{"host":"1.1.1.2","ttl":60}'
~~~

If you query the zone name now, you will get the following response:

~~~ sh
% dig +short skydns.local @localhost
1.1.1.1
1.1.1.2
~~~

### Zone name as AAAA record

If you would like to use `AAAA` records for the zone name too, you can set the following:
~~~
% /skydns/local/skydns/x3 '{"host":"2003::8:1","ttl":60}'
% /skydns/local/skydns/x4 '{"host":"2003::8:2","ttl":60}'
~~~

If you query the zone name for `AAAA` now, you will get the following response:
~~~ sh
% dig +short skydns.local AAAA @localhost
2003::8:1
2003::8:2
~~~

### SRV record

If you would like to use `SRV` records, you can set the following:
~~~
% /skydns/local/skydns/x5 '{"host":"skydns-local.server","ttl":60,"priority":10,"port":8080}'
~~~
Please notice that the key `host` is the `target` in `SRV`, so it should be a domain name.

If you query the zone name for `SRV` now, you will get the following response:

~~~ sh
% dig +short skydns.local SRV @localhost
10 100 8080 skydns-local.server.
~~~

### TXT record

If you would like to use `TXT` records, you can set the following:
~~~
% /skydns/local/skydns/x6 '{"ttl":60,"text":"this is a random text message."}'
~~~

If you query the zone name for `TXT` now, you will get the following response:
~~~ sh
% dig +short skydns.local TXT @localhost
"this is a random text message."
~~~
