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
// The fields fall into two classes, and which class a field belongs to decides
// whether it must appear in the pbconfigentry.InferenceGateway proto message:
//
//   - RENDERED   Consul reads the field and turns it into Envoy configuration
//     (Processor, Failover). Must be in the proto: on a server-managed proxy
//     the entry reaches proxycfg over pbsubscribe.
//   - STORED     Consul only stores and returns the field; the processor fetches
//     it straight from the config-entry HTTP API (PII, AuditLevel,
//     Observability).
//     Deliberately excluded from the proto (ignore-fields).
//
// Adding a field means picking a class. TestInferenceGateway_ProtoRoundTrip
// enforces the choice.
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

	// --- STORED: only the processor reads these ---

	// PII configures per-detector PII detection and redaction. Consul stores and
	// returns it verbatim so the processor reads the same entry Consul renders
	// Envoy from, keeping one source of truth.
	PII *InferenceGatewayPII `json:",omitempty"`

	// AuditLevel is the processor's audit verbosity: full | sampling | off.
	//
	// Deprecated: superseded by Observability.Audit.Level, which validates the value
	// and carries the sampling rate and sink alongside it. Still honoured when no
	// Observability.Audit block is set; AuditLevelOrLegacy resolves the two so the
	// processor reads one field rather than choosing between two.
	AuditLevel string `json:",omitempty" alias:"audit_level"`

	// Observability configures the processor's telemetry pillars — audit, metrics,
	// and tracing. Consul neither renders nor forwards it: the processor reads the
	// entry over the config-entry HTTP API and builds its own exporters, so this is
	// STORED like PII and stays out of the proto.
	Observability *InferenceGatewayObservability `json:",omitempty"`

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

// InferenceGatewayProcessor configures the ext_proc binding to the policy processor.
type InferenceGatewayProcessor struct {
	// UDSPath is the absolute Unix domain socket path the ext_proc filter dials
	// to reach the co-located processor.
	UDSPath string `json:",omitempty" alias:"uds_path"`

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
	// Scope selects which bodies the detectors' actions apply to: request |
	// response | both.
	Scope string `json:",omitempty"`

	// DefaultAction applies to any detector that does not set its own Action:
	// placeholder | mask | block | off.
	DefaultAction string `json:",omitempty" alias:"default_action"`

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
	// Name selects a built-in detector (ssn, credit_card, api_key, email) or names
	// a custom one.
	Name string `json:",omitempty"`
	// Regex is a custom RE2 pattern; empty selects the built-in of Name.
	Regex string `json:",omitempty"`
	// Action is placeholder | mask | block | off.
	Action string `json:",omitempty"`
}

// InferenceGatewayObservability configures the processor's three telemetry pillars
// over a shared request-correlation id. All three are independent and individually
// best-effort: a misconfigured or unreachable sink degrades that pillar, it never
// changes a request's outcome.
//
// Consul stores and returns this verbatim. It is validated here anyway, because a
// bad exporter endpoint or an out-of-range sample rate is a boot failure in the
// processor, and finding it at `consul config write` is cheaper than finding it
// when a gateway refuses to start.
type InferenceGatewayObservability struct {
	Audit   *InferenceGatewayAudit   `json:",omitempty"`
	Metrics *InferenceGatewayMetrics `json:",omitempty"`
	Tracing *InferenceGatewayTracing `json:",omitempty"`
}

// InferenceGatewayAudit configures the compliance audit trail: a two-stage event per
// request, correlated by request id, carrying decision metadata only — never prompt
// or response content.
type InferenceGatewayAudit struct {
	// Level gates emission: full (default) | sampling | off.
	Level string `json:",omitempty"`

	// SampleRate is the [0,1] fraction of ALLOWED traffic emitted under
	// Level = "sampling". Denied requests are always emitted in full.
	SampleRate float64 `json:",omitempty" alias:"sample_rate"`

	Sink *InferenceGatewayAuditSink `json:",omitempty"`
}

// InferenceGatewayAuditSink is the audit destination: JSON lines to a file with
// rotation, or to stdout.
type InferenceGatewayAuditSink struct {
	Type              string `json:",omitempty"`
	Format            string `json:",omitempty"`
	Path              string `json:",omitempty"`
	DeliveryGuarantee string `json:",omitempty" alias:"delivery_guarantee"`
	RotateDuration    string `json:",omitempty" alias:"rotate_duration"`
	RotateBytes       int    `json:",omitempty" alias:"rotate_bytes"`
	RotateMaxFiles    int    `json:",omitempty" alias:"rotate_max_files"`
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
	// upstream rename a config flip rather than a dashboard break.
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
	Port int    `json:",omitempty"`
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

// AuditLevelOrLegacy resolves the audit verbosity from whichever field carries it,
// preferring the Observability block. It exists so the processor never has to know
// that AuditLevel was the older spelling.
func (e *InferenceGatewayConfigEntry) AuditLevelOrLegacy() string {
	if e.Observability != nil && e.Observability.Audit != nil && e.Observability.Audit.Level != "" {
		return e.Observability.Audit.Level
	}
	return e.AuditLevel
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

	e.Processor.FailureMode = strings.ToLower(e.Processor.FailureMode)
	if e.Processor.FailureMode == "" {
		e.Processor.FailureMode = InferenceGatewayFailureModeClosed
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
