// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
)

const (
	// AIGatewayFailureModeClosed rejects the request when the policy processor
	// is unreachable or errors. It is the default.
	AIGatewayFailureModeClosed = "closed"
	// AIGatewayFailureModeOpen lets the request proceed when the processor is
	// unreachable or errors.
	AIGatewayFailureModeOpen = "open"

	// AIGatewayConfigValidationWarn loads shadowed match rules and emits a
	// metric; it is the default.
	AIGatewayConfigValidationWarn = "warn"
	// AIGatewayConfigValidationStrict rejects a config that contains shadowed
	// match rules at load time.
	AIGatewayConfigValidationStrict = "strict"
)

// AIGatewayConfigEntry is the routing policy for one or more inference gateways
// (kind = "inference-gateway"). It binds the gateway's ext_proc filter to a
// co-located policy processor and describes how A2LLM requests are matched and
// routed to model upstreams. It is attached to gateways via ApplyTo and is the
// authoritative source of the gateway's routing behavior (RFC-0002).
type AIGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.AIGateway.
	Kind string

	// Name of the config entry.
	Name string

	// Processor binds the gateway's ext_proc filter to the co-located policy
	// processor over a loopback/UDS socket.
	Processor AIGatewayProcessor

	// ApplyTo lists the inference-gateway service names this policy binds to.
	// An empty list applies the policy to a gateway whose service name equals
	// this entry's Name.
	ApplyTo []string `json:",omitempty"`

	// Routing holds the request-matching and dispatch rules.
	Routing AIGatewayRouting

	// Policy holds cross-cutting request/response policy the co-located processor
	// enforces (PII detection/redaction, audit). Consul does not act on it — it is
	// stored and returned verbatim so the processor reads the same entry it renders
	// Envoy from, keeping one source of truth.
	Policy *AIGatewayPolicy `json:",omitempty"`

	// RateLimit is the token-aware rate-limit policy the co-located processor
	// enforces. Like Policy, Consul stores and returns it verbatim; it is keyed only
	// on trusted sources (SPIFFE identity, config-derived tier, request-body model).
	RateLimit *AIGatewayRateLimit `json:",omitempty"`

	// StateStore locates the shared rate-limit counter as a Consul mesh service.
	// Consul renders it into the gateway's Envoy as an mTLS, intention-gated outbound
	// TCP upstream on LocalBindPort; the processor speaks plain RESP to that local
	// port. Store failover is handled by Envoy (EDS), so the port is stable.
	StateStore *AIGatewayStateStore `json:",omitempty"`

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

// AIGatewayProcessor configures the ext_proc binding to the policy processor.
type AIGatewayProcessor struct {
	// UDSPath is the absolute Unix domain socket path the ext_proc filter dials
	// to reach the co-located processor.
	UDSPath string `json:",omitempty"`

	// FailureMode is "closed" (reject on processor error, the default) or
	// "open" (allow the request through).
	FailureMode string `json:",omitempty"`
}

// AIGatewayRouting holds the Stage 3-6 routing configuration.
type AIGatewayRouting struct {
	// MatchRules are evaluated first-match-wins to select candidate model
	// clusters for a request.
	MatchRules []AIGatewayMatchRule `json:",omitempty"`

	// ComplianceMap AND-filters a match rule's candidates by compliance class.
	ComplianceMap map[string]AIGatewayCompliance `json:",omitempty"`

	// Deprecated: fallback membership + order now come from the catalog (each
	// model's `capabilities` set + `priority.<capability>` meta, gated by
	// intentions). Retained for backward compatibility; unused by rendering.
	FallbackChain []string `json:",omitempty"`

	// Fallback tunes cross-provider failover behavior for capability pools (a
	// capability with two or more discovered members). Membership and per-tier
	// order are NOT here — they come from the catalog.
	Fallback *AIGatewayFallback `json:",omitempty"`

	// Retry and Timeout are the default reliability directives Envoy enforces.
	Retry   *AIGatewayRetry   `json:",omitempty"`
	Timeout *AIGatewayTimeout `json:",omitempty"`

	// Scoring is optional and off by default.
	Scoring *AIGatewayScoring `json:",omitempty"`

	// ConfigValidation is "warn" (default) or "strict"; strict rejects a config
	// with shadowed match rules at load time.
	ConfigValidation string `json:",omitempty"`

	// Reserved forward-compatibility blocks. They accept an empty body but
	// reject any non-empty content at load (deliberate footgun prevention).
	Budget map[string]interface{} `json:",omitempty"`
	Cache  map[string]interface{} `json:",omitempty"`
	Mirror map[string]interface{} `json:",omitempty"`
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

// AIGatewayFallback is the gateway-wide cross-provider failover behavior applied to
// any capability pool. It tunes only HOW Envoy fails a request over across a pool's
// priority tiers; membership and per-tier order come from the catalog (each model's
// capabilities set + priority.<capability> meta). An empty block uses defaults.
type AIGatewayFallback struct {
	// RetryOn lists retriable conditions: HTTP status tokens ("401", "5xx") and
	// Envoy reset triggers ("reset", "connect-failure"). Empty uses the defaults.
	RetryOn []string `json:",omitempty"`
	// MaxTiers caps how many priority tiers one request may walk. 0 = all tiers.
	MaxTiers int `json:",omitempty"`
	// PerTryTimeout bounds each tier attempt (e.g. "30s").
	PerTryTimeout string `json:",omitempty"`
}

// AIGatewayTimeout is the Envoy timeout directive.
type AIGatewayTimeout struct {
	Connect string `json:",omitempty"`
	Request string `json:",omitempty"`
}

// AIGatewayScoring is the optional Stage 5 scorer configuration.
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
// configuration round-trips through Consul to the processor. Field names match the
// processor's config structs (which decode this entry's JSON by Go field name), so
// Consul stores and returns them unchanged.
type AIGatewayPolicy struct {
	// PII configures per-detector PII detection and redaction.
	PII *AIGatewayPII `json:",omitempty"`

	// AuditLevel is the Stage 7 audit verbosity: full | sampling | off.
	AuditLevel string `json:",omitempty"`
}

// AIGatewayPII configures per-detector PII detection and redaction for the
// processor. Consul does not interpret these fields; it carries them to the
// processor verbatim.
type AIGatewayPII struct {
	// Scope selects which bodies the detectors' actions apply to: request |
	// response | both.
	Scope string `json:",omitempty"`

	// DefaultAction applies to any detector that does not set its own Action:
	// placeholder | mask | block | off.
	DefaultAction string `json:",omitempty"`

	// StreamHoldbackBytes is the trailing content the streaming response redactor
	// withholds so PII split across chunk boundaries is caught before release.
	StreamHoldbackBytes int `json:",omitempty"`

	// Mask parameterizes the "mask" action.
	Mask *AIGatewayPIIMask `json:",omitempty"`

	// Detectors are the PII rules to run.
	Detectors []AIGatewayPIIDetector `json:",omitempty"`
}

// AIGatewayPIIMask parameterizes the "mask" redaction action.
type AIGatewayPIIMask struct {
	// Char replaces each redacted alphanumeric character (default "*").
	Char string `json:",omitempty"`
	// KeepLast leaves the trailing KeepLast characters visible.
	KeepLast int `json:",omitempty"`
}

// AIGatewayPIIDetector is one PII rule: a named built-in or a custom Regex, with
// an Action that overrides the policy's DefaultAction.
type AIGatewayPIIDetector struct {
	// Name selects a built-in detector (ssn, credit_card, api_key, email) or names
	// a custom one.
	Name string `json:",omitempty"`
	// Regex is a custom RE2 pattern; empty selects the built-in of Name.
	Regex string `json:",omitempty"`
	// Action is placeholder | mask | block | off.
	Action string `json:",omitempty"`
}

// AIGatewayStateStore locates the rate-limit counter as a Consul mesh service.
// Consul renders an mTLS, intention-gated outbound TCP listener on LocalBindPort
// that proxies to Service; the co-located processor speaks plain RESP to
// 127.0.0.1:LocalBindPort. Distinct from the processor-facing RateLimit policy:
// this drives Envoy rendering (proxycfg/xDS), RateLimit drives the processor.
type AIGatewayStateStore struct {
	// Service is the Consul mesh service name of the counter (e.g. "valkey").
	Service string `json:",omitempty"`

	// LocalBindPort is the loopback port the gateway's Envoy binds and the processor
	// dials. Store endpoints/failover are resolved by Envoy (EDS), so it is stable.
	LocalBindPort int `json:",omitempty"`
}

// AIGatewayRateLimit mirrors the processor's RateLimit block so the token-aware
// limit policy round-trips through Consul to the processor. Field names match the
// processor's config structs (which decode this entry's JSON by Go field name), so
// Consul stores and returns them unchanged. Each budget is a {Count, Unit} pair;
// Unit is second|minute|hour|day (default minute) and its own sliding window is
// derived from it, so requests and tokens can carry independent horizons.
type AIGatewayRateLimit struct {
	Enabled     bool   `json:",omitempty"`
	Enforcement string `json:",omitempty"` // deny (default) | shadow
	Mode        string `json:",omitempty"` // soft (V1, default) | strict (V2, rejected)
	CountMode   string `json:",omitempty"` // total (default) | input | output

	// Dimensions is the enforced subset of {agent, tier, global, model}. An empty
	// list defaults (processor-side) to [tier, global].
	Dimensions []string `json:",omitempty"`

	// DegradeMode is the StateStore-outage posture: fail_closed (default, 503) or
	// fail_open_unlimited (admit, loud audit).
	DegradeMode string `json:",omitempty"`

	// Default is the per-agent (SVID) budget for identities matching no tier.
	Default *AIGatewayLimitPair `json:",omitempty"`

	// Global is the gateway-wide backstop (aggregate provider-account guard).
	Global *AIGatewayLimitPair `json:",omitempty"`

	// TierLimits are per-tier allocations keyed by the SPIFFE-derived tier.
	TierLimits []AIGatewayTierLimit `json:",omitempty"`

	// ModelLimits are optional per-model caps keyed by the request-body model.
	ModelLimits []AIGatewayModelLimit `json:",omitempty"`

	// TierBindings map a trusted identity selector to a named tier. Never
	// header-derived.
	TierBindings []AIGatewayTierBinding `json:",omitempty"`
}

// AIGatewayLimit is a single {Count, Unit} budget: Count events per one Unit-wide
// sliding window. Unit is second|minute|hour|day (default minute); day is a
// rolling 24h window, not a calendar day.
type AIGatewayLimit struct {
	Count int    `json:",omitempty"`
	Unit  string `json:",omitempty"` // second | minute | hour | day (default minute)
}

// AIGatewayLimitPair carries a dimension's two independent budgets: a
// request-count limit and a token limit. Used by Default and Global.
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

// AIGatewayTierBinding maps a trusted identity selector to a named tier. Selectors
// are evaluated most-specific first (exact SPIFFE URI > partition+namespace >
// partition-only); an identity matching none falls to the Default* limits.
type AIGatewayTierBinding struct {
	Tier      string   `json:",omitempty"`
	SPIFFEIDs []string `json:",omitempty"`
	Partition string   `json:",omitempty"`
	Namespace string   `json:",omitempty"`
}

func (e *AIGatewayConfigEntry) GetKind() string                        { return AIGateway }
func (e *AIGatewayConfigEntry) GetName() string                        { return e.Name }
func (e *AIGatewayConfigEntry) GetMeta() map[string]string             { return e.Meta }
func (e *AIGatewayConfigEntry) GetRaftIndex() *RaftIndex               { return &e.RaftIndex }
func (e *AIGatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta { return &e.EnterpriseMeta }
func (e *AIGatewayConfigEntry) GetHash() uint64                        { return e.Hash }
func (e *AIGatewayConfigEntry) SetHash(h uint64)                       { e.Hash = h }

var _ ConfigEntry = (*AIGatewayConfigEntry)(nil)

func (e *AIGatewayConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}
	e.Kind = AIGateway

	e.Processor.FailureMode = strings.ToLower(e.Processor.FailureMode)
	if e.Processor.FailureMode == "" {
		e.Processor.FailureMode = AIGatewayFailureModeClosed
	}

	e.Routing.ConfigValidation = strings.ToLower(e.Routing.ConfigValidation)
	if e.Routing.ConfigValidation == "" {
		e.Routing.ConfigValidation = AIGatewayConfigValidationWarn
	}

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	return nil
}

func (e *AIGatewayConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}
	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	if e.Processor.UDSPath != "" && !strings.HasPrefix(e.Processor.UDSPath, "/") {
		return fmt.Errorf("Processor.UDSPath %q must be an absolute Unix socket path", e.Processor.UDSPath)
	}
	switch e.Processor.FailureMode {
	case "", AIGatewayFailureModeOpen, AIGatewayFailureModeClosed:
	default:
		return fmt.Errorf("Processor.FailureMode %q must be %q or %q",
			e.Processor.FailureMode, AIGatewayFailureModeOpen, AIGatewayFailureModeClosed)
	}

	switch e.Routing.ConfigValidation {
	case "", AIGatewayConfigValidationWarn, AIGatewayConfigValidationStrict:
	default:
		return fmt.Errorf("Routing.ConfigValidation %q must be %q or %q",
			e.Routing.ConfigValidation, AIGatewayConfigValidationWarn, AIGatewayConfigValidationStrict)
	}

	// Reserved blocks must be empty.
	for name, block := range map[string]map[string]interface{}{
		"Budget": e.Routing.Budget,
		"Cache":  e.Routing.Cache,
		"Mirror": e.Routing.Mirror,
	} {
		if len(block) > 0 {
			return fmt.Errorf("Routing.%s is reserved and must be empty", name)
		}
	}

	if e.Routing.Timeout != nil {
		if err := validateOptionalDuration("Routing.Timeout.Connect", e.Routing.Timeout.Connect); err != nil {
			return err
		}
		if err := validateOptionalDuration("Routing.Timeout.Request", e.Routing.Timeout.Request); err != nil {
			return err
		}
	}
	if e.Routing.Retry != nil && e.Routing.Retry.MaxAttempts < 0 {
		return fmt.Errorf("Routing.Retry.MaxAttempts must not be negative")
	}

	for i, rule := range e.Routing.MatchRules {
		if len(rule.Candidates) == 0 {
			return fmt.Errorf("Routing.MatchRules[%d] must list at least one Candidate", i)
		}
	}

	// First-match-wins means a broad earlier rule silently shadows a more
	// specific later rule. In strict mode this is a load-time error.
	if shadows := e.shadowedMatchRules(); len(shadows) > 0 &&
		e.Routing.ConfigValidation == AIGatewayConfigValidationStrict {
		return fmt.Errorf("Routing.MatchRules contains shadowed rules (strict mode): %s",
			strings.Join(shadows, "; "))
	}

	if err := e.validateRateLimit(); err != nil {
		return err
	}

	return nil
}

// validateRateLimit gates the rate-limit policy and its backing StateStore. It is
// the authoritative write-time check; the processor re-validates as defense in
// depth. Keeping it here means bad limits are rejected at `consul config write`.
func (e *AIGatewayConfigEntry) validateRateLimit() error {
	// A StateStore may be declared independently; sanity-check its port whenever set.
	if e.StateStore != nil && e.StateStore.LocalBindPort != 0 &&
		(e.StateStore.LocalBindPort < 1 || e.StateStore.LocalBindPort > 65535) {
		return fmt.Errorf("StateStore.LocalBindPort %d is out of range (1-65535)", e.StateStore.LocalBindPort)
	}

	rl := e.RateLimit
	if rl == nil || !rl.Enabled {
		return nil
	}

	switch rl.Enforcement {
	case "", "deny", "shadow":
	default:
		return fmt.Errorf("RateLimit.Enforcement %q must be \"deny\" or \"shadow\"", rl.Enforcement)
	}
	switch rl.Mode {
	case "", "soft":
	case "strict":
		return fmt.Errorf("RateLimit.Mode \"strict\" is not implemented (use \"soft\")")
	default:
		return fmt.Errorf("RateLimit.Mode %q must be \"soft\"", rl.Mode)
	}
	switch rl.CountMode {
	case "", "total", "input", "output":
	default:
		return fmt.Errorf("RateLimit.CountMode %q must be total|input|output", rl.CountMode)
	}
	switch rl.DegradeMode {
	case "", "fail_closed", "fail_open_unlimited":
	default:
		return fmt.Errorf("RateLimit.DegradeMode %q must be fail_closed|fail_open_unlimited", rl.DegradeMode)
	}

	validDim := map[string]bool{"agent": true, "tier": true, "global": true, "model": true}
	for _, d := range rl.Dimensions {
		if !validDim[d] {
			return fmt.Errorf("RateLimit.Dimensions contains unsupported dimension %q (agent|tier|global|model)", d)
		}
	}

	if err := validateAIGatewayLimitPair("Default", rl.Default); err != nil {
		return err
	}
	if err := validateAIGatewayLimitPair("Global", rl.Global); err != nil {
		return err
	}

	declaredTiers := make(map[string]bool, len(rl.TierLimits))
	for _, tl := range rl.TierLimits {
		if tl.Tier == "" {
			return fmt.Errorf("RateLimit.TierLimit requires a tier name")
		}
		if declaredTiers[tl.Tier] {
			return fmt.Errorf("RateLimit.TierLimit %q is declared more than once", tl.Tier)
		}
		if err := validateAIGatewayLimitUnit(fmt.Sprintf("TierLimit %q.Requests", tl.Tier), tl.Requests); err != nil {
			return err
		}
		if err := validateAIGatewayLimitUnit(fmt.Sprintf("TierLimit %q.Tokens", tl.Tier), tl.Tokens); err != nil {
			return err
		}
		declaredTiers[tl.Tier] = true
	}
	seenModel := make(map[string]bool, len(rl.ModelLimits))
	for _, ml := range rl.ModelLimits {
		if ml.Model == "" {
			return fmt.Errorf("RateLimit.ModelLimit requires a model name")
		}
		if seenModel[ml.Model] {
			return fmt.Errorf("RateLimit.ModelLimit %q is declared more than once", ml.Model)
		}
		if err := validateAIGatewayLimitUnit(fmt.Sprintf("ModelLimit %q.Requests", ml.Model), ml.Requests); err != nil {
			return err
		}
		if err := validateAIGatewayLimitUnit(fmt.Sprintf("ModelLimit %q.Tokens", ml.Model), ml.Tokens); err != nil {
			return err
		}
		seenModel[ml.Model] = true
	}
	for _, tb := range rl.TierBindings {
		if tb.Tier == "" {
			return fmt.Errorf("RateLimit.TierBinding requires a tier name")
		}
		if !declaredTiers[tb.Tier] {
			return fmt.Errorf("RateLimit.TierBinding %q references a tier with no TierLimit", tb.Tier)
		}
	}

	// The counter store must be reachable: StateStore both renders the mesh upstream
	// and carries the LocalBindPort the processor dials (the cross-field invariant).
	if e.StateStore == nil || e.StateStore.Service == "" {
		return fmt.Errorf("RateLimit.Enabled requires a StateStore with a Service name")
	}
	if e.StateStore.LocalBindPort < 1 || e.StateStore.LocalBindPort > 65535 {
		return fmt.Errorf("RateLimit.Enabled requires StateStore.LocalBindPort in range (1-65535)")
	}

	return nil
}

// validateAIGatewayLimitPair validates the {Requests, Tokens} specs of a Default
// or Global block. A nil pair is valid (the dimension is simply absent).
func validateAIGatewayLimitPair(name string, p *AIGatewayLimitPair) error {
	if p == nil {
		return nil
	}
	if err := validateAIGatewayLimitUnit(name+".Requests", p.Requests); err != nil {
		return err
	}
	return validateAIGatewayLimitUnit(name+".Tokens", p.Tokens)
}

// validateAIGatewayLimitUnit checks that a limit's Unit, when set, is one of the
// known windows. An empty Unit is valid (the processor defaults it to minute).
func validateAIGatewayLimitUnit(name string, l *AIGatewayLimit) error {
	if l == nil || l.Unit == "" {
		return nil
	}
	switch l.Unit {
	case "second", "minute", "hour", "day":
		return nil
	}
	return fmt.Errorf("RateLimit %s has invalid Unit %q (second|minute|hour|day)", name, l.Unit)
}

// shadowedMatchRules returns human-readable descriptions of (shadowing,
// shadowed) rule pairs where an earlier rule matches every request a later,
// more specific rule would.
func (e *AIGatewayConfigEntry) shadowedMatchRules() []string {
	var out []string
	rules := e.Routing.MatchRules
	for i := range rules {
		for j := i + 1; j < len(rules); j++ {
			if matchCovers(rules[i].When, rules[j].When) {
				out = append(out, fmt.Sprintf("rule %d shadows rule %d", i, j))
			}
		}
	}
	return out
}

// matchCovers reports whether predicate a matches every request that predicate
// b would (so a placed before b makes b unreachable).
func matchCovers(a, b AIGatewayMatch) bool {
	// Path: a must be unconstrained or identical to b's path.
	if a.Path != "" && a.Path != b.Path {
		return false
	}
	// BodyHas: every token a requires must also be required by b.
	for _, t := range a.BodyHas {
		if !containsString(b.BodyHas, t) {
			return false
		}
	}
	// Identity: each constraint a sets must be unconstrained or equal in b.
	if a.Identity != nil {
		if b.Identity == nil {
			return false
		}
		if !identityFieldCovers(a.Identity.Service, b.Identity.Service) ||
			!identityFieldCovers(a.Identity.Partition, b.Identity.Partition) ||
			!identityFieldCovers(a.Identity.Namespace, b.Identity.Namespace) {
			return false
		}
	}
	return true
}

// identityFieldCovers reports whether constraint a covers b for a single
// identity field. A wildcard "*" or empty a covers anything; otherwise a must
// equal b.
func identityFieldCovers(a, b string) bool {
	return a == "" || a == "*" || a == b
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func validateOptionalDuration(field, value string) error {
	if value == "" {
		return nil
	}
	if _, err := time.ParseDuration(value); err != nil {
		return fmt.Errorf("%s %q is not a valid duration: %w", field, value, err)
	}
	return nil
}

func (e *AIGatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(e.Name, &authzContext)
}

func (e *AIGatewayConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}
