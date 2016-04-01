# etcd

`etcd` enabled reading zone data from an etcd instance. The data in etcd has to be encoded as
a [message](https://github.com/skynetservices/skydns/blob/2fcff74cdc9f9a7dd64189a447ef27ac354b725f/msg/service.go#L26)
like [SkyDNS](https//github.com/skynetservices/skydns).

The etcd middleware makes extensive use of the proxy middleware to forward and query
other servers in the network.

## Syntax

~~~
etcd [zones...]
~~~

* `zones` zones etcd should be authoritative for.

The will default to `/skydns` as the path and the local etcd proxy (http://127.0.0.1:2379).
If no zones are specified the block's zone will be used as the zone.

If you want to `round robin` A and AAAA responses look at the `loadbalance` middleware.

~~~
etcd [zones...] {
    stubzones
    path /skydns
    endpoint endpoint...
    upstream address...
    tls cert key cacert
}
~~~

* `stubzones` enable the stub zones feature.
* `path` the path inside etcd, defaults to "/skydns".
* `endpoint` the etcd endpoints, default to "http://localhost:2397".
* `upstream` upstream resolvers to be used resolve external names found in etcd.
* `tls` followed the cert, key and the CA's cert filenames.

## Examples
