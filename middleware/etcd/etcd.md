# etcd

`etcd` enabled reading zone data from an etcd instance. The data in etcd has to be encoded as
a [message](https://github.com/skynetservices/skydns/blob/2fcff74cdc9f9a7dd64189a447ef27ac354b725f/msg/service.go#L26)
like SkyDNS.

## Syntax

~~~
etcd [endpoint...]
~~~

* `endpoint` is the endpoint of etcd.

The will default to `/skydns` as the path and the local etcd proxy (http://127.0.0.1:2379).

~~~
etcd {
    round_robin
    path /skydns
    endpoint address...
    stubzones
}
~~~

* `round_robin`
* `path` /skydns
* `endpoint` address...
* `stubzones`

## Examples
