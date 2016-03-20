# etcd

`etcd` enabled reading zone data from an etcd instance. The data in etcd has to be encoded as
a [message](https://github.com/skynetservices/skydns/blob/2fcff74cdc9f9a7dd64189a447ef27ac354b725f/msg/service.go#L26)
like [SkyDNS](https//github.com/skynetservices/skydns).

## Syntax

~~~
etcd [zones...]
~~~

* `zones` zones it should be authoritative for.

The will default to `/skydns` as the path and the local etcd proxy (http://127.0.0.1:2379).
If no zones are specified the block's zone will be used as the zone.

If you want to `round robin` A and AAAA responses look at the `round_robin` middleware. optimize
middleware?

~~~
etcd {
    path /skydns
    endpoint endpoint...
    stubzones
}
~~~

* `path` /skydns
* `endpoint` endpoints...
* `stubzones`

## Examples
