# OpenTracing support for gRPC in Python

The `grpcio-opentracing` package makes it easy to add OpenTracing support to 
gRPC-based systems in Python.

## Installation

```
pip install grpcio-opentracing
```

## Getting started

See the below code for basic usage or [examples/trivial](examples/trivial) for a
complete example.

### Client-side usage example

```python
from grpc_opentracing import open_tracing_client_interceptor
from grpc_opentracing.grpcext import intercept_channel

tracer = # some OpenTracing Tracer instance
interceptor = open_tracing_client_interceptor(tracer)
channel = # the grpc.Channel you created to invoke RPCs
channel = intercept_channel(channel, interceptor)

# All future RPC activity involving `channel` will be automatically traced.
```

### Server-side usage example

```python
from grpc_opentracing import open_tracing_server_interceptor
from grpc_opentracing.grpcext import intercept_server

tracer = # some OpenTracing Tracer instance
interceptor = open_tracing_server_interceptor(tracer)
server = # the grpc.Server you created to receive RPCs
server = intercept_server(server, interceptor)

# All future RPC activity involving `server` will be automatically traced.
```

### Integrating with other spans.

`grpcio-opentracing` provides features that let you connect its span with other
tracing spans. On the client-side, you can write a class that derives from
`ActiveSpanSource` and provide it when creating the interceptor.

```python
class CustomActiveSpanSource(ActiveSpanSource):
  def get_active_span(self):
    # your custom method of getting the active span
tracer = # some OpenTracing Tracer instance
interceptor = open_tracing_client_interceptor(
                  tracer,
                  active_span_source=CustomActiveSpanSource)
...
```

On the server-side, the `context` argument passed into your service methods
packages the gRPC span created on the server-side.

```python
class CustomRpcService(...):
  ...
  def Method1(self, request, context):
    span = context.get_active_span()
    ...
```

See [examples/integration](examples/integration) for a complete example.
