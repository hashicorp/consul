# file

`file` enabled reading zone data from a RFC-1035 styled file.

The etcd middleware makes extensive use of the proxy middleware to forward and query
other servers in the network.

## Syntax

~~~
file dbfile [zones...]
~~~

* `dbfile` the database file to read and parse.
* `zones` zones it should be authoritative for. If empty the zones from the configuration block
    are used.

If you want to round robin A and AAAA responses look at the `loadbalance` middleware.

TSIG key configuration is TODO; directive format will change.

~~~
file dbfile [zones... ] {
    transfer out [address...]
}
~~~

* `transfer` enable zone transfers, for now only `transfer out` does something. It enables outgoing
  zone transfers when defined.

## Examples
