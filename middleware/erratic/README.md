# erratic

*erratic* is a middleware useful for testing client behavior. It returns a static response to all
queries, but the responses can be:

* delayed by some duration
* dropped all together
* the truncated bit can be set

The *erratic* middleware will respond to every A or AAAA query. For any other type it will return
a SERVFAIL response. The reply for A will return 192.0.2.53 (see RFC 5737), for AAAA it returns
2001:DB8::53 (see RFC 3849).

## Syntax

~~~ txt
erratic {
    drop [AMOUNT]
    truncate [AMOUNT]
    delay [AMOUNT [DURATION]]
}
~~~

* `drop`: drop 1 per **AMOUNT** of queries, the default is 2.
* `truncate`: truncate 1 per **AMOUNT** of queries, the default is 2.
* `delay`: delay 1 per **AMOUNT** of queries for **DURATION**, the default for **AMOUNT** is 2 and
  the default for **DURATION** is 100ms.

## Examples

~~~ txt
.:53 {
    erratic {
        drop 3
    }
}
~~~

Or even shorter if the defaults suits you. Note this only drops queries, it does not delay them.

~~~ txt
. {
    erratic
}
~~~

Delay 1 in 3 queries for 50ms

~~~ txt
. {
    erratic {
        delay 3 50ms
    }
}
~~~

Delay 1 in 3 and truncate 1 in 5.

~~~ txt
. {
    erratic {
        delay 3 5ms
        truncate 5
    }
}
~~~

Drop every second query.

~~~ txt
. {
    erratic {
        drop 2
        truncate 2
    }
}
~~~
