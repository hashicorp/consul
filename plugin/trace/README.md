# trace

## Name

*trace* - enables OpenTracing-based tracing of DNS requests as they go through the plugin chain.

## Description

With *trace* you enable OpenTracing of how a request flows through CoreDNS.

## Syntax

The simplest form is just:

~~~
trace [ENDPOINT-TYPE] [ENDPOINT]
~~~

* **ENDPOINT-TYPE** is the type of tracing destination. Currently only `zipkin` is supported
  and that is what it defaults to.
* **ENDPOINT** is the tracing destination, and defaults to `localhost:9411`. For Zipkin, if
  ENDPOINT does not begin with `http`, then it will be transformed to `http://ENDPOINT/api/v1/spans`.

With this form, all queries will be traced.

Additional features can be enabled with this syntax:

~~~
trace [ENDPOINT-TYPE] [ENDPOINT] {
	every AMOUNT
	service NAME
	client_server
}
~~~

* `every` **AMOUNT** will only trace one query of each AMOUNT queries. For example, to trace 1 in every
  100 queries, use AMOUNT of 100. The default is 1.
* `service` **NAME** allows you to specify the service name reported to the tracing server.
  Default is `coredns`.
* `client_server` will enable the `ClientServerSameSpan` OpenTracing feature.

## Zipkin
You can run Zipkin on a Docker host like this:

```
docker run -d -p 9411:9411 openzipkin/zipkin
```

## Examples

Use an alternative Zipkin address:

~~~
trace tracinghost:9253
~~~

or

~~~ corefile
. {
    trace zipkin tracinghost:9253
}
~~~

If for some reason you are using an API reverse proxy or something and need to remap
the standard Zipkin URL you can do something like:

~~~
trace http://tracinghost:9411/zipkin/api/v1/spans
~~~

Trace one query every 10000 queries, rename the service, and enable same span:

~~~
trace tracinghost:9411 {
	every 10000
	service dnsproxy
	client_server
}
~~~
