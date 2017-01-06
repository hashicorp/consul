# erratic

*erratic* is a middleware useful for testing client behavior. It returns a static response to all
queries, but the responses can be delayed by a random amount of time or dropped all together, i.e.
no answer at all.

~~~ txt
._<transport>.qname. 0 IN SRV 0 0 <port> .
~~~

The *erratic* middleware will respond to every A or AAAA query. For any other type it will return
a SERVFAIL response. The reply for A will return 192.0.2.53 (see RFC 5737), for AAAA it returns
2001:DB8::53 (see RFC 3849).

## Syntax

~~~ txt
erratic {
    drop AMOUNT
}
~~~

* **AMOUNT** drop 1 per **AMOUNT** of the queries, the default is 2.

## Examples

~~~ txt
.:53 {
    erratic {
        drop 3
    }
}
~~~

Or even shorter if the defaults suits you:

~~~ txt
. {
    erratic
}
~~~

## Bugs

Delaying answers is not implemented.
