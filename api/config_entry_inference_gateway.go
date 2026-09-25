// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

// InferenceGatewayConfigEntry is the routing policy for one or more inference gateways
// (kind = "inference-gateway"). It binds the gateway's ext_proc filter to a
// co-located policy processor and describes how A2LLM requests are matched and
// routed to model upstreams.
type InferenceGatewayConfigEntry struct {
	// Kind must be "inference-gateway".
	Kind string

	// Name of the config entry.
	Name string

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
	// spanning every failover attempt (e.g. "10m"). It applies to both capability
	// and per-model routes. Empty or "0s" disables it, which is the default, so a
	// long or streaming inference response is not cut off by Envoy's 15s default.
	// When set alongside Failover.PerTryTimeout it must be the larger of the two.
	RequestTimeout string `json:",omitempty" alias:"request_timeout"`

	// PII configures per-detector PII detection and redaction. Consul stores and
	// returns it verbatim; only the co-located processor reads it.
	PII *InferenceGatewayPII `json:",omitempty"`

	// Observability configures the processor's telemetry pillars. Consul stores and
	// returns it; the processor reads it and builds its own exporters.
	Observability *InferenceGatewayObservability `json:",omitempty"`

	// Partition is the partition the config entry is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the config entry is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	Meta map[string]string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at.
	CreateIndex uint64

	// ModifyIndex is used for Check-And-Set operations.
	ModifyIndex uint64
}

// InferenceGatewayProcessor configures the ext_proc binding to the policy processor.
//
// The socket path is not here: it is host-local, per-instance, launch-time state
// that a cluster-wide entry cannot express. `consul connect envoy` derives it per
// proxy instance.
type InferenceGatewayProcessor struct {
	FailureMode string `json:",omitempty" alias:"failure_mode"`

	// BodyModelRouting routes on the request body's model instead of a
	// caller-supplied capability header. Consul renders the matching per-model route.
	BodyModelRouting bool `json:",omitempty" alias:"body_model_routing"`
}

// InferenceGatewayFailover is the gateway-wide cross-provider failover behavior applied to
// any capability pool (a capability with two or more discovered members). Membership
// and per-tier order are NOT here — they come from the catalog (each model's
// capabilities set + priority_<capability> meta). An omitted block uses defaults.
type InferenceGatewayFailover struct {
	RetryOn       []string `json:",omitempty" alias:"retry_on"`
	MaxTiers      int      `json:",omitempty" alias:"max_tiers"`
	PerTryTimeout string   `json:",omitempty" alias:"per_try_timeout"`
}

// InferenceGatewayPII configures per-detector PII detection and redaction for the
// processor. Consul stores and returns these fields verbatim.
type InferenceGatewayPII struct {
	Scope               InferenceGatewayPIIScope      `json:",omitempty"`
	DefaultAction       InferenceGatewayPIIAction     `json:",omitempty" alias:"default_action"`
	StreamHoldbackBytes int                           `json:",omitempty" alias:"stream_holdback_bytes"`
	Mask                *InferenceGatewayPIIMask      `json:",omitempty"`
	Detectors           []InferenceGatewayPIIDetector `json:",omitempty"`
}

// InferenceGatewayPIIMask parameterizes the "mask" redaction action.
type InferenceGatewayPIIMask struct {
	Char     string `json:",omitempty"`
	KeepLast int    `json:",omitempty" alias:"keep_last"`
}

// InferenceGatewayPIIDetector is one PII rule: a named built-in or a custom Regex, with
// an Action that overrides PII.DefaultAction. Regex is required unless Name is a
// built-in (api_key, credit_card, email, ssn).
type InferenceGatewayPIIDetector struct {
	Name   string                    `json:",omitempty"`
	Regex  string                    `json:",omitempty"`
	Action InferenceGatewayPIIAction `json:",omitempty"`
}

// InferenceGatewayPIIScope selects which bodies PII detection applies to. Empty
// leaves it to the processor, which defaults to request.
type InferenceGatewayPIIScope string

const (
	InferenceGatewayPIIScopeRequest  InferenceGatewayPIIScope = "request"
	InferenceGatewayPIIScopeResponse InferenceGatewayPIIScope = "response"
	InferenceGatewayPIIScopeBoth     InferenceGatewayPIIScope = "both"
)

// InferenceGatewayPIIAction is what the processor does with a detected match.
// Empty inherits: a detector takes DefaultAction, and DefaultAction the
// processor's default (placeholder).
type InferenceGatewayPIIAction string

const (
	InferenceGatewayPIIActionPlaceholder InferenceGatewayPIIAction = "placeholder"
	InferenceGatewayPIIActionMask        InferenceGatewayPIIAction = "mask"
	InferenceGatewayPIIActionBlock       InferenceGatewayPIIAction = "block"
	InferenceGatewayPIIActionOff         InferenceGatewayPIIAction = "off"
)

// InferenceGatewayObservability configures the processor's metrics and tracing
// pillars. Each is independent and best-effort: a bad sink degrades that pillar,
// it never changes a request's outcome.
type InferenceGatewayObservability struct {
	Metrics *InferenceGatewayMetrics `json:",omitempty"`
	Tracing *InferenceGatewayTracing `json:",omitempty"`
}

// InferenceGatewayMetrics configures OTel metrics export. Enabled is a pointer so an
// unset field stays distinguishable from an explicit false: metrics default to on.
type InferenceGatewayMetrics struct {
	Enabled    *bool                              `json:",omitempty"`
	Prometheus *InferenceGatewayMetricsPrometheus `json:",omitempty"`
	OTLP       *InferenceGatewayOTLPExport        `json:",omitempty"`
}

// InferenceGatewayMetricsPrometheus configures the processor's scrape endpoint,
// which is always served on /metrics. Port is a pointer because 0 is meaningful:
// nil keeps the processor's default port, 0 turns the scrape endpoint off, and
// 1-65535 moves it.
type InferenceGatewayMetricsPrometheus struct {
	Port *int `json:",omitempty"`
}

// InferenceGatewayTracing configures OTel tracing. Off by default.
type InferenceGatewayTracing struct {
	Enabled     bool                        `json:",omitempty"`
	OTLP        *InferenceGatewayOTLPExport `json:",omitempty"`
	SampleRatio float64                     `json:",omitempty" alias:"sample_ratio"`
}

// InferenceGatewayOTLPExport is a shared OTLP exporter target.
type InferenceGatewayOTLPExport struct {
	Endpoint string `json:",omitempty"`
	Insecure bool   `json:",omitempty"`
}

func (e *InferenceGatewayConfigEntry) GetKind() string            { return e.Kind }
func (e *InferenceGatewayConfigEntry) GetName() string            { return e.Name }
func (e *InferenceGatewayConfigEntry) GetPartition() string       { return e.Partition }
func (e *InferenceGatewayConfigEntry) GetNamespace() string       { return e.Namespace }
func (e *InferenceGatewayConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *InferenceGatewayConfigEntry) GetCreateIndex() uint64     { return e.CreateIndex }
func (e *InferenceGatewayConfigEntry) GetModifyIndex() uint64     { return e.ModifyIndex }
