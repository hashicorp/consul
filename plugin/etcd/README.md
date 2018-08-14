# etcd

## Name

*etcd* - enables reading zone data from an etcd version 3 instance.

## Description

The data in etcd instance has to be encoded as
a [message](https://github.com/skynetservices/skydns/blob/2fcff74cdc9f9a7dd64189a447ef27ac354b725f/msg/service.go#L26)
like [SkyDNS](https://github.com/skynetservices/skydns). It should also work just like SkyDNS.

The etcd plugin makes extensive use of the proxy plugin to forward and query other servers in the
network.

## Syntax

~~~
etcd [ZONES...]
~~~

* **ZONES** zones etcd should be authoritative for.

The path will default to `/skydns` the local etcd3 proxy (http://localhost:2379). If no zones are
specified the block's zone will be used as the zone.

If you want to `round robin` A and AAAA responses look at the `loadbalance` plugin.

~~~
etcd [ZONES...] {
    stubzones
    fallthrough [ZONES...]
    path PATH
    endpoint ENDPOINT...
    upstream [ADDRESS...]
    tls CERT KEY CACERT
}
~~~

* `stubzones` enables the stub zones feature. The stubzone is *only* done in the etcd tree located
    under the *first* zone specified.
* `fallthrough` If zone matches but no record can be generated, pass request to the next plugin.
  If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.
* **PATH** the path inside etcd. Defaults to "/skydns".
* **ENDPOINT** the etcd endpoints. Defaults to "http://localhost:2379".
* `upstream` upstream resolvers to be used resolve external names found in etcd (think CNAMEs)
  pointing to external names. If you want CoreDNS to act as a proxy for clients, you'll need to add
  the proxy plugin. If no **ADDRESS** is given, CoreDNS will resolve CNAMEs against itself.
  **ADDRESS** can be an IP address, and IP:port or a string pointing to a file that is structured
  as /etc/resolv.conf.
* `tls` followed by:

    * no arguments, if the server certificate is signed by a system-installed CA and no client cert is needed
    * a single argument that is the CA PEM file, if the server cert is not signed by a system CA and no client cert is needed
    * two arguments - path to cert PEM file, the path to private key PEM file - if the server certificate is signed by a system-installed CA and a client certificate is needed
    * three arguments - path to cert PEM file, path to client private key PEM file, path to CA PEM
      file - if the server certificate is not signed by a system-installed CA and client certificate
      is needed.

## Special Behaviour
CoreDNS etcd plugin leverages directory structure to look for related entries. For example an entry `/skydns/test/skydns/mx` would have entries like `/skydns/test/skydns/mx/a`, `/skydns/test/skydns/mx/b` and so on. Similarly a directory `/skydns/test/skydns/mx1` will have all `mx1` entries.

With etcd3, support for [hierarchial keys are dropped](https://coreos.com/etcd/docs/latest/learning/api.html). This means there are no directories but only flat keys with prefixes in etcd3. To accommodate lookups, etcdv3 plugin now does a lookup on prefix `/skydns/test/skydns/mx/` to search for entries like `/skydns/test/skydns/mx/a` etc, and if there is nothing found on `/skydns/test/skydns/mx/`, it looks for `/skydns/test/skydns/mx` to find entries like `/skydns/test/skydns/mx1`.

This causes two lookups from CoreDNS to etcdv3 in certain cases.

## Migration to `etcdv3` API

With CoreDNS release `1.2.0`, you'll need to migrate existing CoreDNS related data (if any) on your etcd server to etcdv3 API. This is because with `etcdv3` support, CoreDNS can't see the data stored to an etcd server using `etcdv2` API.

Refer this [blog by CoreOS team](https://coreos.com/blog/migrating-applications-etcd-v3.html) to migrate to etcdv3 API.

## Examples

This is the default SkyDNS setup, with everything specified in full:

~~~ corefile
. {
    etcd skydns.local {
        stubzones
        path /skydns
        endpoint http://localhost:2379
        upstream 8.8.8.8:53 8.8.4.4:53
    }
    prometheus
    cache 160 skydns.local
    loadbalance
    proxy . 8.8.8.8:53 8.8.4.4:53
}
~~~

Or a setup where we use `/etc/resolv.conf` as the basis for the proxy and the upstream
when resolving external pointing CNAMEs.

~~~ corefile
. {
    etcd skydns.local {
        path /skydns
        upstream /etc/resolv.conf
    }
    cache 160 skydns.local
    proxy . /etc/resolv.conf
}
~~~

Multiple endpoints are supported as well.

~~~
etcd skydns.local {
    endpoint http://localhost:2379 http://localhost:4001
...
~~~

Before getting started with these examples, please setup `etcdctl` (with `etcdv3` API) as explained [here](https://coreos.com/etcd/docs/latest/dev-guide/interacting_v3.html). This will help you to put sample keys in your etcd server.

If you prefer, you can use `curl` to populate the `etcd` server, but with `curl` the endpoint URL depends on the version of `etcd`. For instance, `etcd v3.2` or before uses only [CLIENT-URL]/v3alpha/* while `etcd v3.5` or later uses [CLIENT-URL]/v3/* . Also, Key and Value must be base64 encoded in the JSON payload. With, `etcdctl` these details are automatically taken care off. You can check [this document](https://github.com/coreos/etcd/blob/master/Documentation/dev-guide/api_grpc_gateway.md#notes) for details.

### Reverse zones

Reverse zones are supported. You need to make CoreDNS aware of the fact that you are also
authoritative for the reverse. For instance if you want to add the reverse for 10.0.0.0/24, you'll
need to add the zone `0.0.10.in-addr.arpa` to the list of zones. Showing a snippet of a Corefile:

~~~
etcd skydns.local 10.0.0.0/24 {
    stubzones
...
~~~

Next you'll need to populate the zone with reverse records, here we add a reverse for
10.0.0.127 pointing to reverse.skydns.local.

~~~
% etcdctl put /skydns/arpa/in-addr/10/0/0/127 '{"host":"reverse.skydns.local."}'
~~~

Querying with dig:

~~~ sh
% dig @localhost -x 10.0.0.127 +short
reverse.skydns.local.
~~~

### Zone name as A record

The zone name itself can be used A record. This behavior can be achieved by writing special entries to the ETCD path of your zone. If your zone is named `skydns.local` for example, you can create an `A` record for this zone as follows:

~~~
% etcdctl put /skydns/local/skydns/ '{"host":"1.1.1.1","ttl":60}'
~~~

If you query the zone name itself, you will receive the created `A` record:

~~~ sh
% dig +short skydns.local @localhost
1.1.1.1
~~~

If you would like to use DNS RR for the zone name, you can set the following:
~~~
% etcdctl put /skydns/local/skydns/x1 '{"host":"1.1.1.1","ttl":"60"}'
% etcdctl put /skydns/local/skydns/x2 '{"host":"1.1.1.2","ttl":"60"}'
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
% etcdctl put /skydns/local/skydns/x3 '{"host":"2003::8:1","ttl":"60"}'
% etcdctl put /skydns/local/skydns/x4 '{"host":"2003::8:2","ttl":"60"}'
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
% etcdctl put /skydns/local/skydns/x5 '{"host":"skydns-local.server","ttl":60,"priority":10,"port":8080}'
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
% etcdctl put /skydns/local/skydns/x6 '{"ttl":60,"text":"this is a random text message."}'
~~~

If you query the zone name for `TXT` now, you will get the following response:
~~~ sh
% dig +short skydns.local TXT @localhost
"this is a random text message."
~~~
