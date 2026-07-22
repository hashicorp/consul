// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package api

// AIGatewayConfigEntry is the routing policy for one or more inference gateways
// (kind = "inference-gateway"). It binds the gateway's ext_proc filter to a
// co-located policy processor and describes how A2LLM requests are matched and
// routed to model upstreams.
type AIGatewayConfigEntry struct {
	// Kind must be "ai-gateway".
	Kind string

	// Name of the config entry.
	Name string

	// Processor binds the gateway's ext_proc filter to the co-located policy
	// processor over a loopback/UDS socket.
	Processor AIGatewayProcessor

	// ApplyTo lists the inference-gateway service names this policy binds to.
	ApplyTo []string `json:",omitempty"`

	// Routing holds the request-matching and dispatch rules.
	Routing AIGatewayRouting

	// Policy holds cross-cutting request/response policy (PII, audit) the
	// co-located processor enforces. Consul carries it verbatim to the processor.
	Policy *AIGatewayPolicy `json:",omitempty"`

	// RateLimit is the token-aware rate-limit policy the co-located processor
	// enforces. Consul carries it verbatim to the processor and renders it into the
	// gateway's Envoy (listener metadata) for hot-reload.
	RateLimit *AIGatewayRateLimit `json:",omitempty"`

	// StateStore locates the shared rate-limit counter as a Consul mesh service.
	// Consul renders it into the gateway's Envoy as an mTLS, intention-gated outbound
	// TCP upstream on LocalBindPort.
	StateStore *AIGatewayStateStore `json:",omitempty"`

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

// AIGatewayProcessor configures the ext_proc binding to the policy processor.
type AIGatewayProcessor struct {
	UDSPath     string `json:",omitempty"`
	FailureMode string `json:",omitempty"`
}

// AIGatewayRouting holds the request-routing configuration.
type AIGatewayRouting struct {
	MatchRules    []AIGatewayMatchRule           `json:",omitempty"`
	ComplianceMap map[string]AIGatewayCompliance `json:",omitempty"`
	// Deprecated: fallback membership + order now come from the catalog (each
	// model's capabilities set + priority.<capability> meta). Retained for
	// backward compatibility; unused by rendering.
	FallbackChain []string `json:",omitempty"`
	// Fallback tunes cross-provider failover behavior for capability pools.
	Fallback         *AIGatewayFallback     `json:",omitempty"`
	Retry            *AIGatewayRetry        `json:",omitempty"`
	Timeout          *AIGatewayTimeout      `json:",omitempty"`
	Scoring          *AIGatewayScoring      `json:",omitempty"`
	ConfigValidation string                 `json:",omitempty"`
	Budget           map[string]interface{} `json:",omitempty"`
	Cache            map[string]interface{} `json:",omitempty"`
	Mirror           map[string]interface{} `json:",omitempty"`
}

// AIGatewayFallback is the gateway-wide cross-provider failover behavior applied to
// any capability pool (a capability with two or more discovered members). Membership
// and per-tier order are NOT here — they come from the catalog (each model's
// capabilities set + priority.<capability> meta). An omitted block uses defaults.
type AIGatewayFallback struct {
	RetryOn       []string `json:",omitempty"`
	MaxTiers      int      `json:",omitempty"`
	PerTryTimeout string   `json:",omitempty"`
}

// AIGatewayMatchRule selects candidate clusters for matching requests.
type AIGatewayMatchRule struct {
	When                AIGatewayMatch `json:",omitempty"`
	RequireCapabilities []string       `json:",omitempty"`
	Candidates          []string       `json:",omitempty"`
	FallbackChain       []string       `json:",omitempty"`
}

// AIGatewayMatch is the predicate for a match rule.
type AIGatewayMatch struct {
	Path     string                  `json:",omitempty"`
	BodyHas  []string                `json:",omitempty"`
	Identity *AIGatewayIdentityMatch `json:",omitempty"`
}

// AIGatewayIdentityMatch matches on the calling agent's SPIFFE identity.
type AIGatewayIdentityMatch struct {
	Service   string `json:",omitempty"`
	Partition string `json:",omitempty"`
	Namespace string `json:",omitempty"`
}

// AIGatewayCompliance constrains candidate clusters for a compliance class.
type AIGatewayCompliance struct {
	AllowedRegions  []string `json:",omitempty"`
	AllowedClusters []string `json:",omitempty"`
}

// AIGatewayRetry is the Envoy retry directive.
type AIGatewayRetry struct {
	MaxAttempts int      `json:",omitempty"`
	RetryOn     []string `json:",omitempty"`
}

// AIGatewayTimeout is the Envoy timeout directive.
type AIGatewayTimeout struct {
	Connect string `json:",omitempty"`
	Request string `json:",omitempty"`
}

// AIGatewayScoring is the optional scorer configuration.
type AIGatewayScoring struct {
	Scorers       []string                  `json:",omitempty"`
	WeightedSplit []AIGatewayWeightedTarget `json:",omitempty"`
}

// AIGatewayWeightedTarget is a weighted cluster in a scoring split.
type AIGatewayWeightedTarget struct {
	Cluster string
	Weight  int
}

// AIGatewayPolicy mirrors the policy processor's Policy block so the PII and audit
// configuration round-trips through Consul to the processor.
type AIGatewayPolicy struct {
	PII        *AIGatewayPII `json:",omitempty"`
	AuditLevel string        `json:",omitempty"`
}

// AIGatewayPII configures per-detector PII detection and redaction for the
// processor. Consul carries these fields verbatim.
type AIGatewayPII struct {
	Scope               string                 `json:",omitempty"`
	DefaultAction       string                 `json:",omitempty"`
	StreamHoldbackBytes int                    `json:",omitempty"`
	Mask                *AIGatewayPIIMask      `json:",omitempty"`
	Detectors           []AIGatewayPIIDetector `json:",omitempty"`
}

// AIGatewayPIIMask parameterizes the "mask" redaction action.
type AIGatewayPIIMask struct {
	Char     string `json:",omitempty"`
	KeepLast int    `json:",omitempty"`
}

// AIGatewayPIIDetector is one PII rule: a named built-in or a custom Regex, with
// an Action that overrides the policy's DefaultAction.
type AIGatewayPIIDetector struct {
	Name   string `json:",omitempty"`
	Regex  string `json:",omitempty"`
	Action string `json:",omitempty"`
}

// AIGatewayStateStore locates the rate-limit counter as a Consul mesh service.
type AIGatewayStateStore struct {
	Service       string `json:",omitempty"`
	LocalBindPort int    `json:",omitempty"`
}

// AIGatewayRateLimit mirrors the processor's RateLimit block so the token-aware
// limit policy round-trips through Consul to the processor. Each budget is a
// {Count, Unit} pair; Unit is second|minute|hour|day (default minute).
type AIGatewayRateLimit struct {
	Enabled      bool                   `json:",omitempty"`
	Enforcement  string                 `json:",omitempty"`
	Mode         string                 `json:",omitempty"`
	CountMode    string                 `json:",omitempty"`
	Dimensions   []string               `json:",omitempty"`
	DegradeMode  string                 `json:",omitempty"`
	Default      *AIGatewayLimitPair    `json:",omitempty"`
	Global       *AIGatewayLimitPair    `json:",omitempty"`
	TierLimits   []AIGatewayTierLimit   `json:",omitempty"`
	ModelLimits  []AIGatewayModelLimit  `json:",omitempty"`
	TierBindings []AIGatewayTierBinding `json:",omitempty"`
}

// AIGatewayLimit is a single {Count, Unit} budget.
type AIGatewayLimit struct {
	Count int    `json:",omitempty"`
	Unit  string `json:",omitempty"`
}

// AIGatewayLimitPair carries a dimension's request-count and token budgets.
type AIGatewayLimitPair struct {
	Requests *AIGatewayLimit `json:",omitempty"`
	Tokens   *AIGatewayLimit `json:",omitempty"`
}

// AIGatewayTierLimit is a per-tier allocation keyed by the SPIFFE-derived tier.
type AIGatewayTierLimit struct {
	Tier                   string          `json:",omitempty"`
	Requests               *AIGatewayLimit `json:",omitempty"`
	Tokens                 *AIGatewayLimit `json:",omitempty"`
	MaxCompletionTokensCap int             `json:",omitempty"`
}

// AIGatewayModelLimit is a per-model cap keyed by the request-body model.
type AIGatewayModelLimit struct {
	Model    string          `json:",omitempty"`
	Requests *AIGatewayLimit `json:",omitempty"`
	Tokens   *AIGatewayLimit `json:",omitempty"`
}

// AIGatewayTierBinding maps a trusted identity selector to a named tier.
type AIGatewayTierBinding struct {
	Tier      string   `json:",omitempty"`
	SPIFFEIDs []string `json:",omitempty"`
	Partition string   `json:",omitempty"`
	Namespace string   `json:",omitempty"`
}

func (e *AIGatewayConfigEntry) GetKind() string            { return e.Kind }
func (e *AIGatewayConfigEntry) GetName() string            { return e.Name }
func (e *AIGatewayConfigEntry) GetPartition() string       { return e.Partition }
func (e *AIGatewayConfigEntry) GetNamespace() string       { return e.Namespace }
func (e *AIGatewayConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *AIGatewayConfigEntry) GetCreateIndex() uint64     { return e.CreateIndex }
func (e *AIGatewayConfigEntry) GetModifyIndex() uint64     { return e.ModifyIndex }
