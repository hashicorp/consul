# pprof

*pprof* publishes runtime profiling data at endpoints under /debug/pprof. You can visit
 `/debug/pprof`
on your site for an index of the available endpoints. By default it will listen on localhost:6053.

> This is a debugging tool. Certain requests (such as collecting execution traces) can be slow. If
> you use pprof on a live site, consider restricting access or enabling it only temporarily.

For more information, please see [Go's pprof
documentation](https://golang.org/pkg/net/http/pprof/s://golang.org/pkg/net/http/pprof/) and read
[Profiling Go Programs](https://blog.golang.org/profiling-go-programs).

There is not configuration.

## Syntax

~~~
pprof
~~~

## Examples

Enable pprof endpoints:

~~~
pprof
~~~
