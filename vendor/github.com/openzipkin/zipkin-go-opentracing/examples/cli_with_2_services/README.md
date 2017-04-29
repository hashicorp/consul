## Zipkin tracing using OpenTracing API

This directory contains a super simple command line client and two HTTP services
which are instrumented using the OpenTracing API using the
[zipkin-go-opentracing](https://github.com/openzipkin/zipkin-go-opentracing)
tracer.

The code is a quick hack to solely demonstrate the usage of
[OpenTracing](http://opentracing.io) with a [Zipkin](http://zipkin.io) backend.

```
note: the examples will only compile with Go 1.7 or higher
```

## Usage:

Build `svc1`, `svc2` and `cli` with `make` and start both compiled services
found in the newly created `build` subdirectory.

When you call the `cli` program it will trigger two calls to `svc1` of which one
call will be proxied from `svc1` over to `svc2` to handle the method and by that
generating a couple of spans across services.

Methods have been instrumented with some examples of
[OpenTracing](http://opentracing.io) Tags which will be transformed into
[Zipkin](http://zipkin.io) binary annotations and
[OpenTracing](http://opentracing.io) LogEvents which will be transformed into
[Zipkin](http://zipkin.io) annotations.

The most interesting piece of code is found in `examples/middleware` which is
kind of the missing link for people looking for a tracing framework. I advise
you to seriously look into using [Go kit](https://gokit.io) and use its
abstractions and OpenTracing middleware with which this Tracer is fully
compatible, instead of rolling your own. If you still want to roll your own you
can use `examples/middleware` as a starting point.
