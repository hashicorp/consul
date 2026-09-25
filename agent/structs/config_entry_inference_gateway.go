// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
)

const (
	// InferenceGatewayFailureModeClosed rejects the request when the policy processor
	// is unreachable or errors. It is the default.
	InferenceGatewayFailureModeClosed = "closed"
	// InferenceGatewayFailureModeOpen lets the request proceed when the processor is
	// unreachable or errors.
	InferenceGatewayFailureModeOpen = "open"
)

// InferenceGatewayConfigEntry is the routing policy for one or more inference gateways
// (kind = "inference-gateway"). It binds the gateway's ext_proc filter to a
// co-located policy processor and describes how A2LLM requests are matched and
// routed to model upstreams. A gateway binds to the entry whose Name equals its
// own service name, and the entry is the authoritative source of the gateway's
// processor binding, failover tuning, and content policy (RFC-0002).
//
// The fields fall into two classes by what Consul DOES with them. Every field
// crosses the wire: this entry has exactly one path out of Consul, and a field the
// proto drops cannot reach anything downstream at all.
//
//   - RENDERED   Consul reads the field and turns it into Envoy configuration
//     (Processor, Failover, RequestTimeout).
//   - FORWARDED  Consul does not interpret the field but carries it to the
//     co-located policy processor as Envoy listener metadata
//     (PII, Observability).
//
// Both classes must appear in the pbconfigentry.InferenceGateway proto message,
// because on a server-managed proxy the entry reaches proxycfg over pbsubscribe.
// There is deliberately no third class for fields the processor fetches itself:
// the processor does not talk to Consul. Config reaches it only through Envoy, so
// Consul resolves the entry once and hands over the result — the same shape
// consul-dataplane gets from GetEnvoyBootstrapParams, and the same reason
// proxy-defaults is merged server-side rather than by the proxy. Two consumers
// resolving one entry independently is what the -ext-proc-config-entry flag turned
// into a silent total outage.
//
// Adding a field means picking a class. TestInferenceGateway_ProtoFieldsClassified
// enforces that every field has one; TestInferenceGateway_ProtoRoundTrip then
// enforces that the classification actually holds across the wire.
type InferenceGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.InferenceGateway.
	Kind string

	// Name of the config entry.
	Name string

	// --- RENDERED: Consul turns these into Envoy configuration ---

	// Processor binds the gateway's ext_proc filter to the co-located policy
	// processor over a loopback/UDS socket.
	Processor InferenceGatewayProcessor

	// Failover tunes cross-provider failover for capability pools (a capability
	// with two or more discovered members). It governs only HOW Envoy fails a
	// request over across a pool's priority tiers; WHICH models serve a capability
	// and in what order comes from the catalog (each model's `capabilities` set and
	// `priority_<capability>` meta), gated by intentions.
	Failover *InferenceGatewayFailover `json:",omitempty"`

	// RequestTimeout is the overall deadline for one request through the gateway,
	// from the request arriving to the response ending, spanning every failover
	// attempt (e.g. "10m"). Rendered as the route timeout on BOTH the capability and
	// the per-model routes, so the two routing modes share one deadline.
	//
	// Empty or "0s" disables it, and that is the default. Envoy's own default of 15s
	// is set for API traffic and cuts off an inference response - a long completion,
	// or a stream still delivering tokens - mid-body. With it disabled a request is
	// bounded instead by Failover.PerTryTimeout for each attempt and by Envoy's
	// stream idle timeout for a stalled stream.
	RequestTimeout string `json:",omitempty" alias:"request_timeout"`

	// --- FORWARDED: carried to the processor as Envoy listener metadata ---

	// PII configures per-detector PII detection and redaction. Consul does not
	// interpret it; it renders the block into the gateway listener's
	// consul.inference.policy metadata, which the processor reads off the
	// xds.listener_metadata request attribute.
	PII *InferenceGatewayPII `json:",omitempty"`

	// Observability configures the processor's telemetry pillars — metrics and
	// tracing. Forwarded the same way as PII: Consul builds no exporters, it only
	// carries the block, so the processor and Envoy are configured from one read of
	// one entry.
	Observability *InferenceGatewayObservability `json:",omitempty"`

	Meta map[string]string `json:",omitempty"`

	// Hash covers the whole entry and is computed by HashConfigEntry on the
	// state-store copy at `consul config write` and snapshot restore. Every field
	// crosses the pbsubscribe stream, so an entry delivered to proxycfg re-hashes to
	// the value it carries; treat it as opaque anyway, since recomputing it is
	// needless work.
	Hash uint64 `json:",omitempty" hash:"ignore"`

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

// InferenceGatewayProcessor configures the ext_proc binding to the policy processor.
type InferenceGatewayProcessor struct {
	// NOTE: the ext_proc socket path is deliberately NOT here. It is host-local,
	// per-instance, launch-time state: the processor must bind it before Envoy can
	// connect, and two instances of this gateway on one host need two paths. A
	// config entry is cluster-wide and replicated, so it can express neither. The
	// path is derived per proxy instance by `consul connect envoy`, which is also
	// where the processor's other launch concerns (-ext-proc-bin,
	// -ext-proc-log-level) already live.

	// FailureMode is "closed" (reject on processor error, the default) or
	// "open" (allow the request through).
	FailureMode string `json:",omitempty" alias:"failure_mode"`

	// BodyModelRouting selects routing on the request body's model rather than on a
	// caller-supplied capability header. When true the processor promotes the body
	// model onto x-inference-model and clears the route cache, and Consul renders a
	// per-model route matching that header — the two halves are useless apart, which
	// is why this is RENDERED rather than merely stored.
	//
	// Default false: capability routing needs no per-model route table and keeps the
	// caller's intent explicit.
	BodyModelRouting bool `json:",omitempty" alias:"body_model_routing"`
}

// InferenceGatewayFailover is the gateway-wide cross-provider failover behavior applied to
// any capability pool. It tunes only HOW Envoy fails a request over across a pool's
// priority tiers; membership and per-tier order come from the catalog (each model's
// capabilities set + priority.<capability> meta). An empty block uses defaults.
type InferenceGatewayFailover struct {
	// RetryOn lists retriable conditions: HTTP status tokens ("401", "5xx") and
	// Envoy reset triggers ("reset", "connect-failure"). Empty uses the defaults.
	RetryOn []string `json:",omitempty" alias:"retry_on"`
	// MaxTiers caps how many priority tiers one request may walk. 0 = all tiers.
	MaxTiers int `json:",omitempty" alias:"max_tiers"`
	// PerTryTimeout bounds each tier attempt (e.g. "30s").
	PerTryTimeout string `json:",omitempty" alias:"per_try_timeout"`
}

// InferenceGatewayPII configures per-detector PII detection and redaction for the
// processor. Consul does not interpret these fields; it stores and returns them
// verbatim.
type InferenceGatewayPII struct {
	// Scope selects which bodies the detectors' actions apply to. Empty leaves it
	// to the processor, which defaults to request.
	Scope InferenceGatewayPIIScope `json:",omitempty"`

	// DefaultAction applies to any detector that does not set its own Action.
	// Empty leaves it to the processor, which defaults to placeholder.
	DefaultAction InferenceGatewayPIIAction `json:",omitempty" alias:"default_action"`

	// StreamHoldbackBytes is the trailing content the streaming response redactor
	// withholds so PII split across chunk boundaries is caught before release.
	StreamHoldbackBytes int `json:",omitempty" alias:"stream_holdback_bytes"`

	// Mask parameterizes the "mask" action.
	Mask *InferenceGatewayPIIMask `json:",omitempty"`

	// Detectors are the PII rules to run.
	Detectors []InferenceGatewayPIIDetector `json:",omitempty"`
}

// InferenceGatewayPIIMask parameterizes the "mask" redaction action.
type InferenceGatewayPIIMask struct {
	// Char replaces each redacted alphanumeric character (default "*").
	Char string `json:",omitempty"`
	// KeepLast leaves the trailing KeepLast characters visible.
	KeepLast int `json:",omitempty" alias:"keep_last"`
}

// InferenceGatewayPIIDetector is one PII rule: a named built-in or a custom Regex, with
// an Action that overrides PII.DefaultAction.
type InferenceGatewayPIIDetector struct {
	// Name selects a built-in detector (see inferenceBuiltinPIIDetectors) or, with a
	// Regex, names a custom one for placeholder text and logs.
	Name string `json:",omitempty"`
	// Regex is a custom RE2 pattern. It is required unless Name is a built-in, in
	// which case leaving it empty selects the built-in (a built-in can do more than
	// match a pattern: credit_card also checks the Luhn digit). Setting it on a
	// built-in name replaces the built-in with the pattern.
	Regex string `json:",omitempty"`
	// Action overrides PII.DefaultAction for this detector. Empty inherits it.
	Action InferenceGatewayPIIAction `json:",omitempty"`
}

// InferenceGatewayPIIScope selects which bodies PII detection applies to.
type InferenceGatewayPIIScope string

const (
	InferenceGatewayPIIScopeRequest  InferenceGatewayPIIScope = "request"
	InferenceGatewayPIIScopeResponse InferenceGatewayPIIScope = "response"
	InferenceGatewayPIIScopeBoth     InferenceGatewayPIIScope = "both"
)

// InferenceGatewayPIIAction is what the processor does with a detected match.
type InferenceGatewayPIIAction string

const (
	// InferenceGatewayPIIActionPlaceholder replaces a match with [REDACTED_<NAME>].
	InferenceGatewayPIIActionPlaceholder InferenceGatewayPIIAction = "placeholder"
	// InferenceGatewayPIIActionMask masks a match per PII.Mask, keeping its shape.
	InferenceGatewayPIIActionMask InferenceGatewayPIIAction = "mask"
	// InferenceGatewayPIIActionBlock rejects the request or response outright.
	InferenceGatewayPIIActionBlock InferenceGatewayPIIAction = "block"
	// InferenceGatewayPIIActionOff disables the detector.
	InferenceGatewayPIIActionOff InferenceGatewayPIIAction = "off"
)

// InferenceGatewayObservability configures the processor's telemetry pillars over
// a shared request-correlation id. Both are independent and individually
// best-effort: a misconfigured or unreachable sink degrades that pillar, it never
// changes a request's outcome.
//
// Consul stores and returns this verbatim. It is validated here anyway, because a
// bad exporter endpoint or an out-of-range sample rate is a boot failure in the
// processor, and finding it at `consul config write` is cheaper than finding it
// when a gateway refuses to start.
//
// The compliance audit trail is deliberately not here: it is out of scope for the
// initial release, so there is no half-specified audit surface to support.
type InferenceGatewayObservability struct {
	Metrics *InferenceGatewayMetrics `json:",omitempty"`
	Tracing *InferenceGatewayTracing `json:",omitempty"`
}

// InferenceGatewayMetrics configures OTel metrics export. Prometheus pull is the
// default so a collector is never a dependency; OTLP push is added when an endpoint
// is set.
type InferenceGatewayMetrics struct {
	// Enabled is a pointer so an unset field is distinguishable from an explicit
	// false: metrics default to ON, and a block written only to set a port must not
	// silently turn them off.
	Enabled    *bool                              `json:",omitempty"`
	Prometheus *InferenceGatewayMetricsPrometheus `json:",omitempty"`
	OTLP       *InferenceGatewayOTLPExport        `json:",omitempty"`

	// SemconvSchema pins the OpenTelemetry semantic-conventions version the emitted
	// gen_ai.* names are drawn from. That vocabulary is pre-1.0, so pinning keeps an
	// upstream rename a config flip rather than a dashboard break. Only the version
	// the processor implements is accepted - see inferenceImplementedSemconvSchema.
	SemconvSchema string `json:",omitempty" alias:"semconv_schema"`

	// CustomLabels promotes an allowlisted, low-cardinality set of tenant metadata
	// keys onto the gateway_* instruments. This is the only sanctioned path for extra
	// labels; anything unbounded here is a cardinality incident.
	CustomLabels []string `json:",omitempty" alias:"custom_labels"`
}

// InferenceGatewayMetricsPrometheus configures the processor's scrape endpoint.
// Omitting the block keeps the default port; setting Port = 0 turns the scrape
// endpoint off while leaving any OTLP push running.
type InferenceGatewayMetricsPrometheus struct {
	Port int `json:",omitempty"`

	// Path is retained for wire compatibility but is not configurable: the processor
	// serves the scrape endpoint on a fixed path, so the only accepted values are
	// empty and that path. See inferenceFixedPrometheusPath.
	Path string `json:",omitempty"`
}

// InferenceGatewayTracing configures OTel tracing. Off by default; when enabled it
// exports over OTLP and joins Envoy's trace via the inbound traceparent.
type InferenceGatewayTracing struct {
	Enabled     bool                        `json:",omitempty"`
	OTLP        *InferenceGatewayOTLPExport `json:",omitempty"`
	SampleRatio float64                     `json:",omitempty" alias:"sample_ratio"`
}

// InferenceGatewayOTLPExport is a shared OTLP exporter target for metrics or traces.
type InferenceGatewayOTLPExport struct {
	Endpoint string `json:",omitempty"`
	Insecure bool   `json:",omitempty"`
}

func (e *InferenceGatewayConfigEntry) GetKind() string            { return InferenceGateway }
func (e *InferenceGatewayConfigEntry) GetName() string            { return e.Name }
func (e *InferenceGatewayConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *InferenceGatewayConfigEntry) GetRaftIndex() *RaftIndex   { return &e.RaftIndex }
func (e *InferenceGatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return &e.EnterpriseMeta
}
func (e *InferenceGatewayConfigEntry) GetHash() uint64  { return e.Hash }
func (e *InferenceGatewayConfigEntry) SetHash(h uint64) { e.Hash = h }

var _ ConfigEntry = (*InferenceGatewayConfigEntry)(nil)

func (e *InferenceGatewayConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}
	e.Kind = InferenceGateway

	// Case-fold every enum-valued field, not just FailureMode. These are all
	// closed value sets compared against lower-case constants, so normalizing one
	// and not the others is what makes FailureMode: "CLOSED" acceptable while
	// PII.Scope: "Request" is rejected — an inconsistency with no reason behind it.
	e.Processor.FailureMode = strings.ToLower(e.Processor.FailureMode)
	if e.Processor.FailureMode == "" {
		e.Processor.FailureMode = InferenceGatewayFailureModeClosed
	}

	if e.PII != nil {
		e.PII.Scope = InferenceGatewayPIIScope(strings.ToLower(string(e.PII.Scope)))
		e.PII.DefaultAction = InferenceGatewayPIIAction(strings.ToLower(string(e.PII.DefaultAction)))
		for i := range e.PII.Detectors {
			e.PII.Detectors[i].Action = InferenceGatewayPIIAction(strings.ToLower(string(e.PII.Detectors[i].Action)))
		}
	}

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	return nil
}

// Validate rejects the entry on Consul CE: the inference gateway is a Consul
// Enterprise feature. CE carries the types so API consumers (such as consul-k8s)
// compile against the same surface, but it neither stores nor renders the entry.
func (e *InferenceGatewayConfigEntry) Validate() error {
	return fmt.Errorf("inference-gateway is a consul enterprise feature")
}

func (e *InferenceGatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(e.Name, &authzContext)
}

func (e *InferenceGatewayConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}
