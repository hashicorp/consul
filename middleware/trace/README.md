# trace

This module enables OpenTracing-based tracing of DNS requests as they go through the
middleware chain.

## Syntax

~~~
trace [ENDPOINT-TYPE] [ENDPOINT]
~~~

For each server you which to trace.

It optionally takes the ENDPOINT-TYPE and ENDPOINT. The ENDPOINT-TYPE defaults to
`zipkin` and the ENDPOINT to `localhost:9411`. A single argument will be interpreted as
a Zipkin ENDPOINT.

The only ENDPOINT-TYPE supported so far is `zipkin`. You can run Zipkin on a Docker host
like this:

```
docker run -d -p 9411:9411 openzipkin/zipkin
```

For Zipkin, if ENDPOINT does not begin with `http`, then it will be transformed to
`http://ENDPOINT/api/v1/spans`.

## Examples

Use an alternative Zipkin address:

~~~
trace tracinghost:9253
~~~

or

~~~
trace zipkin tracinghost:9253
~~~

If for some reason you are using an API reverse proxy or something and need to remap
the standard Zipkin URL you can do something like:

~~~
trace http://tracinghost:9411/zipkin/api/v1/spans
~~~
