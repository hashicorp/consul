# lifecycle

`github.com/hashicorp/consul/lifecycle` is a small, dependency-free Go module that gives a
process (a Consul server, a client agent, consul-dataplane) one place to answer:
**"what state is component X in right now, and what changed?"**

It is in-memory and per process. It is unrelated to Consul's replicated raft FSM: nothing here is
persisted or shared between processes, and it starts empty on every start.

## Concepts

| Concept | What it is |
|---|---|
| `Exchange` | The per-process registry. Create one at startup, pass it where needed. A `nil` `*Exchange` is a valid no-op, so the dependency can be optional. |
| `Component` | Anything with a lifecycle. Named `<entity>/<component>[/<instance>]`, for example `server/state-store`, `agent/streaming-store`, `dataplane/xds`, `server/peerstream/<peer-id>`. |
| `ComponentSpec` | The state machine a component registers: its **own** states, the allowed transitions, and for each state a shared `Phase`. |
| `Phase` | A small, closed vocabulary (`Starting`, `Ready`, `Degraded`, `Transitioning`, `Failed`, `Stopped`) so other components can reason about a component without knowing its states. |
| `Status` | Current state, phase, generation, since, reason and attributes. |
| `Generation` | Increases each time a component becomes Ready after its data or configuration was replaced (a restore, a reload). Listeners use it to reset anything tied to the old generation. |
| `ControlEvent` | Emitted for every register, transition, update and removal. Carries a per-Exchange sequence number and a schema version. |
| `Gate` / `Lease` | A consumer declares what it needs (`ex.Require("server/state-store", lifecycle.PhaseReady)`) and takes a short lease while it works. A disruptive transition waits for the leases already held. |
| `Observer` | A pluggable sink for lifecycle notifications. Logging and metrics are built in. |

## Add a component

```go
const (
    connecting lifecycle.StateName = "Connecting"
    streaming  lifecycle.StateName = "Streaming"
    resyncing  lifecycle.StateName = "Resyncing"
)

h, err := ex.Register(lifecycle.ComponentSpec{
    Name:    "agent/streaming-store",
    Initial: connecting,
    States: map[lifecycle.StateName]lifecycle.StateSpec{
        connecting: {Phase: lifecycle.PhaseStarting,      Next: []lifecycle.StateName{streaming}},
        streaming:  {Phase: lifecycle.PhaseReady,         Next: []lifecycle.StateName{resyncing}},
        resyncing:  {Phase: lifecycle.PhaseTransitioning, Next: []lifecycle.StateName{streaming},
                     MaxDuration: time.Minute}, // reported as stuck after a minute
    },
})
```

For common shapes use a preset, which returns a normal spec you can copy and extend:

- `lifecycle.Simple(name)`: Starting, Ready, Degraded, Stopping, Stopped, Failed.
- `lifecycle.Restorable(name)`: Booting, Serving, Restoring, RestoreFailed. Used by `server/state-store`.
- `lifecycle.Reloadable(name)`: Starting, Ready, Reloading, Failed. Each reload bumps the generation.

Report changes with `h.Transition(state, lifecycle.WithReason(...), lifecycle.WithAttributes(...))`. A transition the
spec does not allow returns `ErrInvalidTransition` and is counted in `consul.lifecycle.invalid_transition`: it is a bug.
Components that come and go (per peer, per upstream) call `h.Deregister()` when they go.

### Choosing a phase

Phases are deliberately closed. Do not add new ones. Put component-specific meaning in the state name and in
attributes. Use `Transitioning` for states in which the component is being replaced under its consumers, because
`BeginTransition` requires it.

## Gate work on a component

```go
gate := ex.Require("server/state-store", lifecycle.PhaseReady)

lease, err := gate.Acquire(ctx, lifecycle.FailFast)
if err != nil {
    // *lifecycle.NotReadyError: tell the caller to retry
}
defer lease.Release()
// ... read the state ...
```

- **`FailFast` never blocks**, so it is safe to call while holding other locks.
- **`Wait`** blocks until the gate opens. Allowed for a single component only, and never while holding other locks.
- Custom conditions: `ex.RequireFunc(name, "description", func(Status) bool)`. The predicate must be pure and fast.
- Several components: `gate.And(...)`. Leases are always taken in component-name order.

### The component that is being changed

```go
done, err := h.BeginTransition(ctx, lifecycle.StateRestoring, lifecycle.WithReason("snapshot restore"))
// New leases are refused; leases taken earlier have been waited for (bounded by the drain timeout).
// ... replace the state ...
err = done(lifecycle.StateServing)
```

## Rules

1. **Keep leases short.** Never hold one for the lifetime of a stream. A long-lived stream is closed by the component
   that replaced the data, and the client resubscribes.
2. **Do not hold other locks while waiting** in `Acquire(Wait)` or `BeginTransition`. This is what keeps the Exchange
   from creating a new lock-ordering problem. `FailFast` is exempt.
3. **The Exchange never calls out while holding its own locks.** Observers and predicates run without them. Do not rely
   on any ordering between your own locks and the Exchange's.
4. **Low write volume.** Transitions are for restores, reloads, connects and disconnects, not for every request. Do not
   register per-request components.
5. **Drain is bounded.** If leases are still held when the drain timeout passes, the transition proceeds, a warning is
   logged and `consul.lifecycle.drain_timeout` increases. Make sure holders release.

## Read state

- `ex.Status(name)`, `ex.Statuses()`: current level.
- `ex.Subscribe(ctx, opts...)`: a snapshot first, then ordered events. Options: `WithComponents`, `WithPrefix`, `WithBuffer`.
  A reader that falls behind never blocks writers: it gets a fresh snapshot with `Resync` set instead of the dropped events.
- `ex.WaitFor(ctx, name, phases...)`.

## Add an output

Implement `lifecycle.Observer` (embed `lifecycle.NoopObserver` to implement only what you need) and pass
`lifecycle.WithObserver(...)` to `NewExchange`. Observers are called from one goroutine, in order, outside all locks; a
panic is recovered and logged. Built in: `WithLogger` and `WithMetrics` (see `agent/lifecycleadapter` for the Consul
adapter and the metric definitions).

## Memory

Memory is bounded by design: one status per component (no history), per-subscriber queues with resync on overflow, a
bounded observer queue that drops and counts (`consul.lifecycle.observer_dropped`), capped attributes (16 keys, 64 byte
keys, 256 byte values), a component limit (10,000) and counter-based leases. All limits are options.

## Testing your component

`lifecycletest` provides a manual `Clock`, a `Recorder` observer and assertions (`AssertTransitions`, `AssertNeverStuck`,
`AssertDrained`). `example_test.go` shows a component with custom states, a custom requirement and a custom observer,
using only the public API.
