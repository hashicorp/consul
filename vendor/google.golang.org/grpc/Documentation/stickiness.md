# Stickiness

With load balancer, each RPC pick a different backend based on the load
balancing policy. Stickiness policies try to preserve peers for the duration of
a session, so that RPCs with the same stickiness key will be directed to the
same server.

Note that there's only "soft" stickiness now, which means RPCs with the same
stickienss key could still be sent to different servers. If stickiness is
critical for the system, server side application level handling is still
necessary.

## Stickiness Key

A stickiness key works as the session id. RPCs with the same stickiness key will
be assigned to the same backend.

Stickiness key is set as part of the custom metadata.

## Enable stickiness

Stickiness can be enabled by setting `stickinessKey` field in [service
config](https://github.com/grpc/grpc/blob/master/doc/service_config.md).

```json
{
  "stickinessKey": "sessionid"
}
```

The value `sesseionid` will be used as the key of the metadata entry that
defines the stickiness key for each RPC.

## Send RPC with stickiness

To set the stickiness key for an RPC, set the corresponding metadata. The
following RPC will be sent with stickiness key `session1`.

```go
// "sessionid" is the metadata key specified by service config, "session1" is
// the stickiness key for this RPC.
md := metadata.Paris("sessionid", "session1")

ctx := metadata.NewOutgoingContext(context.Background(), md)
resp, err := client.SomeRPC(ctx, req)
```
