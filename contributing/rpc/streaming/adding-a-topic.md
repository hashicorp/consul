
# Adding a new topic to streaming

This document is a guide for adding a new streaming topic.

1. Add the name of the topic to the [proto/pbsubscribe/subscribe.proto Topic enum][1].
   Run `make proto` to generate the Go code from the protobuf.
2. Add a `FromChanges` function to the list of change functions in
   [agent/consul/state.processDBChanges][2]. The `FromChanges` function should examine the
   list of `Changes` and return a list of events that subscriptions would need to update
   their view.
3. Add a snapshot function to [agent/consul/state.newSnapshotHandlers][3]. The snapshot
   function should produce a set of events to create the initial state of the view.
   Generally these are all "create" events.
4. Add a new `Payload` type, similar to [agent/consul/state.EventPayloadCheckServiceNode][6].
   This type will be used in the `Payload` field of the event.
5. Create the protobuf for the payload, and add the `Payload` type to the `oneof` in
   [proto/pbsubscrube/subscribe.proto Event.Payload][7]. This may require creating other
   protobuf types as well, to encode anything in the payload. Run `make proto` to generate
   the Go code from the protobuf.
6. Add another case to [agent/rpc/subscribe.setPayload][8] to convert from the Payload
   type in `state`, to the protobuf type. This may require either writing or generating a
   function to convert between the types.
7. Add a new cache-type that uses [agent/submatview.Materializer][4] similar to
   [agent/cache-types/streaming_health_services.go][5].


[1]: https://github.com/hashicorp/consul/blob/v1.9.4/proto/pbsubscribe/subscribe.proto#L37-L45
[2]: https://github.com/hashicorp/consul/blob/v1.9.4/agent/consul/state/memdb.go#L188-L192
[3]: https://github.com/hashicorp/consul/blob/v1.9.4/agent/consul/state/memdb.go#L205-L209
[4]: https://github.com/hashicorp/consul/blob/v1.9.4/agent/submatview/materializer.go#L76
[5]: https://github.com/hashicorp/consul/blob/v1.9.4/agent/cache-types/streaming_health_services.go
[6]: https://github.com/hashicorp/consul/blob/v1.9.4/agent/consul/state/catalog_events.go#L12-L46
[7]: https://github.com/hashicorp/consul/blob/v1.9.4/proto/pbsubscribe/subscribe.proto#L95-L117
[8]: https://github.com/hashicorp/consul/blob/v1.9.4/agent/rpc/subscribe/subscribe.go#L161-L168
