# etcd

## Name

*etcd* - enables reading zone data from an etcd instance.

## Description

The data in etcd has to be encoded as
a [message](https://github.com/skynetservices/skydns/blob/2fcff74cdc9f9a7dd64189a447ef27ac354b725f/msg/service.go#L26)
like [SkyDNS](https://github.com/skynetservices/skydns). It should also work just like SkyDNS.

The etcd plugin makes extensive use of the proxy plugin to forward and query other servers in the
network.

## Syntax

~~~
etcd [ZONES...]
~~~

* **ZONES** zones etcd should be authoritative for.

The path will default to `/skydns` the local etcd proxy (http://localhost:2379). If no zones are
specified the block's zone will be used as the zone.

If you want to `round robin` A and AAAA responses look at the `loadbalance` plugin.

~~~
etcd [ZONES...] {
    stubzones
    fallthrough [ZONES...]
    path PATH
    endpoint ENDPOINT...
    upstream ADDRESS...
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
* **ENDPOINT** the etcd endpoints. Defaults to "http://localhost:2397".
* `upstream` upstream resolvers to be used resolve external names found in etcd (think CNAMEs)
  pointing to external names. If you want CoreDNS to act as a proxy for clients, you'll need to add
  the proxy plugin. **ADDRESS** can be an IP address, and IP:port or a string pointing to a file
  that is structured as /etc/resolv.conf.
* `tls` followed by:

    * no arguments, if the server certificate is signed by a system-installed CA and no client cert is needed
    * a single argument that is the CA PEM file, if the server cert is not signed by a system CA and no client cert is needed
    * two arguments - path to cert PEM file, the path to private key PEM file - if the server certificate is signed by a system-installed CA and a client certificate is needed
    * three arguments - path to cert PEM file, path to client private key PEM file, path to CA PEM
      file - if the server certificate is not signed by a system-installed CA and client certificate
      is needed.

## Examples

This is the default SkyDNS setup, with everying specified in full:

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
% curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/arpa/in-addr/10/0/0/127 \
    -d value='{"host":"reverse.skydns.local."}'
~~~

Querying with dig:

~~~ sh
% dig @localhost -x 10.0.0.127 +short
reverse.skydns.local.
~~~

## Bugs

Only the etcdv2 protocol is supported.
