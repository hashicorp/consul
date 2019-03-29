# pprof

## Name

*pprof* - publishes runtime profiling data at endpoints under `/debug/pprof`.

## Description

You can visit `/debug/pprof` on your site for an index of the available endpoints. By default it
will listen on localhost:6053.

This is a debugging tool. Certain requests (such as collecting execution traces) can be slow. If
you use pprof on a live server, consider restricting access or enabling it only temporarily.

This plugin can only be used once per Server Block.

## Syntax

~~~ txt
pprof [ADDRESS]
~~~

Optionally pprof takes an address; the default is `localhost:6053`.

An extra option can be set with this extended syntax:

~~~ txt
pprof [ADDRESS] {
   block [RATE]
}
~~~

* `block` option enables block profiling, **RATE** defaults to 1. **RATE** must be a positive value.
  See [Diagnostics, chapter profiling](https://golang.org/doc/diagnostics.html) and
  [runtime.SetBlockProfileRate](https://golang.org/pkg/runtime/#SetBlockProfileRate) for what block
  profiling entails.

## Examples

Enable a pprof endpoint:

~~~
. {
    pprof
}
~~~

And use the pprof tool to get statistics: `go tool pprof http://localhost:6053`.

Listen on an alternate address:

~~~ txt
. {
    pprof 10.9.8.7:6060
}
~~~

Listen on an all addresses on port 6060, and enable block profiling

~~~ txt
. {
    pprof :6060 {
       block
    }
}
~~~

## Also See

See [Go's pprof documentation](https://golang.org/pkg/net/http/pprof/) and [Profiling Go
Programs](https://blog.golang.org/profiling-go-programs).

See [runtime.SetBlockProfileRate](https://golang.org/pkg/runtime/#SetBlockProfileRate) for
background on block profiling.
