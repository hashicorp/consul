# chaos

`chaos`

## Syntax

~~~
chaos [version] [authors...]
~~~

* `version` the version to return, defaults to CoreDNS.
* `authors` what authors to return. No default.

## Examples

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
