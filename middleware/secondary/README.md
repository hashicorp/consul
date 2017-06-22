# secondary

*secondary* enables serving a zone retrieved from a primary server.

## Syntax

~~~
secondary [ZONES...]
~~~

* **ZONES** zones it should be authoritative for. If empty, the zones from the configuration block
    are used. Note that without a remote address to *get* the zone from, the above is not that useful.

A working syntax would be:

~~~
secondary [zones...] {
    transfer from ADDRESS
    transfer to ADDRESS
    upstream ADDRESS...
}
~~~

* `transfer from` specifies from which address to fetch the zone. It can be specified multiple times;
    if one does not work, another will be tried.
* `transfer to` can be enabled to allow this secondary zone to be transferred again.
* `upstream` defines upstream resolvers to be used resolve external names found (think CNAMEs)
  pointing to external names. This is only really useful when CoreDNS is configured as a proxy, for
  normal authoritative serving you don't need *or* want to use this.

## Examples

~~~
secondary example.org {
    transfer from 10.0.1.1
    transfer from 10.1.2.1
}
~~~
