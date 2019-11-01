# transfer

## Name

*transfer* - answer zone transfers requests for compatible authoritative
plugins.

## Description

This plugin answers zone transfers for authoritative plugins that implement
`transfer.Transferer`.

Transfer answers AXFR requests and IXFR requests with AXFR fallback if the
zone has changed.

Notifies are not currently supported.

## Syntax

~~~
transfer [ZONE...] {
  to HOST...
}
~~~

* **ZONES** The zones *transfer* will answer zone requests for. If left blank,
  the zones are inherited from the enclosing server block. To answer zone
  transfers for a given zone, there must be another plugin in the same server
  block that serves the same zone, and implements `transfer.Transferer`.

* `to ` **HOST...** The hosts *transfer* will transfer to. Use `*` to permit
  transfers to all hosts.
