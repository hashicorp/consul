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

	// FallbackChain is the default priority-ordered cluster list used when a
	// match rule does not specify its own.
	FallbackChain []string `json:",omitempty"`

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

	return nil
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
