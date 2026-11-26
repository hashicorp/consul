# Scenario: intentions

Registers plain (non-Connect) services and three intentions covering every
`Action` branch the Intentions list and create/edit form render, including a
full Layer 7 permission (path, methods, headers).

| Source | Destination | Action | Covers |
|---|---|---|---|
| `web` | `api` | Allow | Plain Allow intention. |
| `*` (All Services) | `payments` | Deny | Wildcard source → "All Services (*)" rendering. |
| `web` | `payments` | *(blank — Application Aware)* | One Layer 7 permission: Deny, `PathPrefix /admin`, `Methods [GET, POST]`, header `X-Debug` exactly `"true"`, header `X-Trace-Id` present. |

`checkout` is registered but left with no intentions — a free service the
create-form's "type a new destination" flow can target without colliding with
a seeded pair.

`payments` and `checkout` both carry a `service-defaults` config entry
(`Protocol: "http"`) — **required** before a destination can carry Layer 7
permissions. Without it, Consul rejects the intention with a 500
(`has protocol "tcp", which is incompatible with L7 intentions permissions`)
that only surfaces after clicking Save in the UI, with no client-side warning.
Found by driving the real form against a real agent; encoded here so the
scenario — and any test that builds a fresh L7 intention against `checkout` —
always seeds/saves cleanly. Any test that invents its *own* ad-hoc destination
name for an L7 intention needs to register this config entry itself first
(see `tests/intentions/permissions.spec.ts`'s downgrade-warning test).

- Does **not** require the Connect CA (`requiresConnect: false`) — intentions
  and config entries don't need sidecars.
- UI: `http://localhost:4200/ui/dc1/intentions`
- Also exercised: `http://localhost:4200/ui/dc1/services/api/intentions`
  (service-scoped tab — should show only the `web → api` and `* → payments`
  rows, since `api` is a possible match for the wildcard source).
- Tests: [`tests/intentions/`](../../tests/intentions/).
