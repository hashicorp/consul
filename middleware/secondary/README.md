# secondary

`secondary` enables serving a zone retrieved from a primary server.

## Syntax

~~~
secondary [zones...]
~~~

* `zones` zones it should be authoritative for. If empty, the zones from the configuration block
    are used. Not that with an remote address to *get* the zone the above is not that useful.

A working syntax would be:

~~~
secondary [zones...] {
    transfer from address
    [transfer to address]
}
~~~

* `transfer from` tell from which address to fetch the zone. It can be specified multiple time,
    if one does not work another will be tried.
* `transfer to` can be enabled to allow this secondary zone to be transfered again.

## Examples

~~~
secondary [zones...] {
    transfer from 10.0.1.1
    transfer from 10.1.2.1
}
~~~
