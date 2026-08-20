// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
)

// ServiceAIRole is the discriminator for the inline `ai` block on a service
// definition. Exactly one role-specific sub-block is populated based on its
// value. See the CAMP design summary for the full schema.
type ServiceAIRole string

const (
	// ServiceAIRoleInferenceModel registers the service as a mesh-discoverable
	// inference target (internal or, via a terminating gateway, managed).
	ServiceAIRoleInferenceModel ServiceAIRole = "inference-model"

	// ServiceAIRoleMCPServer registers the service as an MCP (Model Context
	// Protocol) tooling target.
	ServiceAIRoleMCPServer ServiceAIRole = "mcp-server"

	// ServiceAIRoleAgent registers the service as an AI agent workload.
	ServiceAIRoleAgent ServiceAIRole = "ai-agent"
)

// Supported enum values for role-specific fields.
const (
	aiProtocolOpenAI      = "openai"
	aiProtocolAnthropic   = "anthropic"
	aiProtocolPassthrough = "passthrough"

	aiTransportStreamableHTTP = "streamable-http"
	aiTransportSSE            = "sse"
	aiTransportStdio          = "stdio"

	aiAuthTypeBearer      = "bearer"
	aiSecretProviderVault = "vault"
)

// ServiceAI is the inline `ai` block added to the standard Consul service
// definition. It carries a Role discriminator and exactly one role-specific
// sub-block. Registering the service and its AI semantics together lets the
// connect-injector attach the correct specialized sidecar in one pass.
type ServiceAI struct {
	// Role determines which role-specific sub-block is valid.
	Role ServiceAIRole `json:",omitempty"`

	// Exactly one of the following is set, matching Role.
	InferenceModel *AIInferenceModel `json:",omitempty" bexpr:"-"`
	MCPServer      *AIMCPServer      `json:",omitempty" bexpr:"-"`
	Agent          *AIAgent          `json:",omitempty" bexpr:"-"`
}

// AIInferenceModel is the role-specific config for ServiceAIRoleInferenceModel.
type AIInferenceModel struct {
	// Protocol is the API dialect the model speaks: openai | anthropic |
	// passthrough.
	Protocol string `json:",omitempty"`

	// Path is the OpenAI-compatible base path (e.g. "/v1").
	Path string `json:",omitempty"`

	// Auth references the credential used to reach a managed/public model.
	// It is nil for internally hosted (mesh) models.
	Auth *AIAuth `json:",omitempty"`

	// Defaults are optional request-shaping defaults.
	Defaults *AIModelDefaults `json:",omitempty"`
}

// AIModelDefaults carries optional request defaults for an inference model.
type AIModelDefaults struct {
	MaxTokens   int     `json:",omitempty"`
	Temperature float64 `json:",omitempty"`
}

// AIMCPServer is the role-specific config for ServiceAIRoleMCPServer.
type AIMCPServer struct {
	// Transport is the MCP transport: streamable-http | sse | stdio.
	Transport string `json:",omitempty"`

	// Path is the MCP endpoint path (e.g. "/mcp").
	Path string `json:",omitempty"`

	// ProtocolVersion is the MCP spec version this server implements.
	ProtocolVersion string `json:",omitempty"`

	// Auth references the credential used to reach an external MCP server.
	// It is nil for internally hosted (mesh) MCP servers.
	Auth *AIAuth `json:",omitempty"`
}

// AIAgent is the role-specific config for ServiceAIRoleAgent.
type AIAgent struct {
	// Inference describes the specialization(s) the agent requires when
	// calling the inference gateway.
	Inference *AIAgentInference `json:",omitempty"`

	// MCP describes the agent's MCP egress: the dedicated outbound listener
	// port and any human-in-the-loop (HITL) approval configuration.
	MCP *AIAgentMCP `json:",omitempty"`

	// RateLimits are per-agent tool-call limits.
	RateLimits *AIAgentRateLimits `json:",omitempty"`

	// Interceptor configures the co-located governance interceptor the sidecar
	// reaches over loopback for request/response middleware.
	Interceptor *AIAgentInterceptor `json:",omitempty"`
}

// AIAgentInference describes the inference specialization for an agent.
type AIAgentInference struct {
	// Specialization is sent as the x-inference-specialization request header.
	Specialization []string `json:",omitempty"`

	// Vendor, when set, is sent as the x-inference-vendor request header.
	Vendor string `json:",omitempty"`
}

// AIAgentMCP is the MCP egress configuration for an agent.
type AIAgentMCP struct {
	// Port overrides the dedicated MCP outbound listener port the agent dials
	// for tool calls (loopback /mcp). Defaults to 15101 when unset.
	Port int `json:",omitempty"`

	// HITL is the optional human-in-the-loop approval configuration.
	HITL *AIAgentMCPHITL `json:",omitempty"`
}

// AIAgentMCPHITL is the human-in-the-loop approval configuration for MCP tool
// calls. The agent runs a plain-HTTP server on the loopback Port; the
// interceptor pushes approval requests for critical tools and blocks until it
// receives a verdict.
type AIAgentMCPHITL struct {
	// Port is the loopback port the agent's HITL HTTP server listens on.
	// Defaults to 16101 when unset.
	Port int `json:",omitempty"`

	// ApprovalTimeout is how long the interceptor waits for the agent's verdict
	// before failing closed (rejecting the tool call).
	ApprovalTimeout string `json:",omitempty"`
}

// AIAgentRateLimits are per-agent tool-call rate limits.
type AIAgentRateLimits struct {
	ToolCallsPerMinute int `json:",omitempty"`
	ToolCallsPerHour   int `json:",omitempty"`
}

// AIAgentInterceptor configures the co-located governance interceptor the
// sidecar reaches over plaintext loopback.
type AIAgentInterceptor struct {
	// Port is the loopback port the interceptor listens on. Defaults to 21101
	// when unset.
	Port int `json:",omitempty"`
}

// AIAuth references a credential used by a managed inference model or external
// MCP server. The secret value is never inlined; only a reference is stored.
type AIAuth struct {
	// Type is the auth scheme. Only "bearer" is currently supported.
	Type string `json:",omitempty"`

	// Header is the request header the credential is injected into.
	Header string `json:",omitempty"`

	// Secret references where the credential is sourced from.
	Secret *AISecret `json:",omitempty"`
}

// AISecret is a reference to a secret stored in an external provider. It never
// holds the literal secret value.
type AISecret struct {
	// Provider is the secret backend. Only "vault" is currently supported.
	Provider string `json:",omitempty"`

	// Path is the provider path the secret is read from.
	Path string `json:",omitempty"`

	// Field is the field within the secret to use.
	Field string `json:",omitempty"`
}

// Clone returns a deep copy of the AI block. It is nil-safe. A deep copy is
// required because ServiceAI is carried on ServiceNode, which is held in the
// shared Raft state store; PartialClone must hand out a fully independent copy
// so that mutating one entry cannot corrupt another.
func (a *ServiceAI) Clone() *ServiceAI {
	if a == nil {
		return nil
	}

	out := &ServiceAI{Role: a.Role}

	if a.InferenceModel != nil {
		im := *a.InferenceModel
		im.Auth = a.InferenceModel.Auth.clone()
		if a.InferenceModel.Defaults != nil {
			d := *a.InferenceModel.Defaults
			im.Defaults = &d
		}
		out.InferenceModel = &im
	}

	if a.MCPServer != nil {
		ms := *a.MCPServer
		ms.Auth = a.MCPServer.Auth.clone()
		out.MCPServer = &ms
	}

	if a.Agent != nil {
		agent := &AIAgent{}
		if a.Agent.Inference != nil {
			inf := &AIAgentInference{Vendor: a.Agent.Inference.Vendor}
			if a.Agent.Inference.Specialization != nil {
				inf.Specialization = make([]string, len(a.Agent.Inference.Specialization))
				copy(inf.Specialization, a.Agent.Inference.Specialization)
			}
			agent.Inference = inf
		}
		if a.Agent.MCP != nil {
			mcp := &AIAgentMCP{Port: a.Agent.MCP.Port}
			if a.Agent.MCP.HITL != nil {
				hitl := &AIAgentMCPHITL{
					Port:            a.Agent.MCP.HITL.Port,
					ApprovalTimeout: a.Agent.MCP.HITL.ApprovalTimeout,
				}
				mcp.HITL = hitl
			}
			agent.MCP = mcp
		}
		if a.Agent.RateLimits != nil {
			rl := *a.Agent.RateLimits
			agent.RateLimits = &rl
		}
		if a.Agent.Interceptor != nil {
			ic := *a.Agent.Interceptor
			agent.Interceptor = &ic
		}
		out.Agent = agent
	}

	return out
}

func (a *AIAuth) clone() *AIAuth {
	if a == nil {
		return nil
	}
	out := *a
	if a.Secret != nil {
		s := *a.Secret
		out.Secret = &s
	}
	return &out
}

// UnmarshalJSON adds snake_case aliases for the multi-word child keys of the
// `ai` block so that config-file (snake_case) and API (CamelCase) bodies parse
// identically. CamelCase values take precedence when both are present.
func (t *ServiceAI) UnmarshalJSON(data []byte) error {
	type Alias ServiceAI
	aux := &struct {
		InferenceModelSnake *AIInferenceModel `json:"inference_model"`
		MCPServerSnake      *AIMCPServer      `json:"mcp_server"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
	if t.InferenceModel == nil {
		t.InferenceModel = aux.InferenceModelSnake
	}
	if t.MCPServer == nil {
		t.MCPServer = aux.MCPServerSnake
	}
	return nil
}

// UnmarshalJSON adds a snake_case alias for protocol_version.
func (t *AIMCPServer) UnmarshalJSON(data []byte) error {
	type Alias AIMCPServer
	aux := &struct {
		ProtocolVersionSnake string `json:"protocol_version"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
	if t.ProtocolVersion == "" {
		t.ProtocolVersion = aux.ProtocolVersionSnake
	}
	return nil
}

// UnmarshalJSON adds a snake_case alias for max_tokens.
func (t *AIModelDefaults) UnmarshalJSON(data []byte) error {
	type Alias AIModelDefaults
	aux := &struct {
		MaxTokensSnake int `json:"max_tokens"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
	if t.MaxTokens == 0 {
		t.MaxTokens = aux.MaxTokensSnake
	}
	return nil
}

// UnmarshalJSON adds a snake_case alias for rate_limits.
func (t *AIAgent) UnmarshalJSON(data []byte) error {
	type Alias AIAgent
	aux := &struct {
		RateLimitsSnake *AIAgentRateLimits `json:"rate_limits"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
	if t.RateLimits == nil {
		t.RateLimits = aux.RateLimitsSnake
	}
	return nil
}

// UnmarshalJSON adds a snake_case alias for approval_timeout.
func (t *AIAgentMCPHITL) UnmarshalJSON(data []byte) error {
	type Alias AIAgentMCPHITL
	aux := &struct {
		ApprovalTimeoutSnake string `json:"approval_timeout"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
	if t.ApprovalTimeout == "" {
		t.ApprovalTimeout = aux.ApprovalTimeoutSnake
	}
	return nil
}

// UnmarshalJSON adds snake_case aliases for tool_calls_per_minute and
// tool_calls_per_hour.
func (t *AIAgentRateLimits) UnmarshalJSON(data []byte) error {
	type Alias AIAgentRateLimits
	aux := &struct {
		ToolCallsPerMinuteSnake int `json:"tool_calls_per_minute"`
		ToolCallsPerHourSnake   int `json:"tool_calls_per_hour"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
	if t.ToolCallsPerMinute == 0 {
		t.ToolCallsPerMinute = aux.ToolCallsPerMinuteSnake
	}
	if t.ToolCallsPerHour == 0 {
		t.ToolCallsPerHour = aux.ToolCallsPerHourSnake
	}
	return nil
}

// ToAPI converts a structs.ServiceAI into its public api.AgentServiceAI twin.
// It is nil-safe (a nil receiver returns nil) and performs a deep copy so the
// returned value never aliases the source slices/pointers. Only secret
// references (provider/path/field) are copied; no resolved secret value exists
// on either side to leak.
func (a *ServiceAI) ToAPI() *api.AgentServiceAI {
	if a == nil {
		return nil
	}
	return &api.AgentServiceAI{
		Role:           string(a.Role),
		InferenceModel: a.InferenceModel.toAPI(),
		MCPServer:      a.MCPServer.toAPI(),
		Agent:          a.Agent.toAPI(),
	}
}

func (m *AIInferenceModel) toAPI() *api.AgentAIInferenceModel {
	if m == nil {
		return nil
	}
	return &api.AgentAIInferenceModel{
		Protocol: m.Protocol,
		Path:     m.Path,
		Auth:     m.Auth.toAPI(),
		Defaults: m.Defaults.toAPI(),
	}
}

func (d *AIModelDefaults) toAPI() *api.AgentAIModelDefaults {
	if d == nil {
		return nil
	}
	return &api.AgentAIModelDefaults{
		MaxTokens:   d.MaxTokens,
		Temperature: d.Temperature,
	}
}

func (s *AIMCPServer) toAPI() *api.AgentAIMCPServer {
	if s == nil {
		return nil
	}
	return &api.AgentAIMCPServer{
		Transport:       s.Transport,
		Path:            s.Path,
		ProtocolVersion: s.ProtocolVersion,
		Auth:            s.Auth.toAPI(),
	}
}

func (a *AIAgent) toAPI() *api.AgentAIAgent {
	if a == nil {
		return nil
	}
	return &api.AgentAIAgent{
		Inference:   a.Inference.toAPI(),
		MCP:         a.MCP.toAPI(),
		RateLimits:  a.RateLimits.toAPI(),
		Interceptor: a.Interceptor.toAPI(),
	}
}

func (i *AIAgentInference) toAPI() *api.AgentAIAgentInference {
	if i == nil {
		return nil
	}
	out := &api.AgentAIAgentInference{Vendor: i.Vendor}
	if i.Specialization != nil {
		out.Specialization = make([]string, len(i.Specialization))
		copy(out.Specialization, i.Specialization)
	}
	return out
}

func (m *AIAgentMCP) toAPI() *api.AgentAIAgentMCP {
	if m == nil {
		return nil
	}
	return &api.AgentAIAgentMCP{
		Port: m.Port,
		HITL: m.HITL.toAPI(),
	}
}

func (h *AIAgentMCPHITL) toAPI() *api.AgentAIAgentMCPHITL {
	if h == nil {
		return nil
	}
	return &api.AgentAIAgentMCPHITL{
		Port:            h.Port,
		ApprovalTimeout: h.ApprovalTimeout,
	}
}

func (r *AIAgentRateLimits) toAPI() *api.AgentAIAgentRateLimits {
	if r == nil {
		return nil
	}
	return &api.AgentAIAgentRateLimits{
		ToolCallsPerMinute: r.ToolCallsPerMinute,
		ToolCallsPerHour:   r.ToolCallsPerHour,
	}
}

func (i *AIAgentInterceptor) toAPI() *api.AgentAIAgentInterceptor {
	if i == nil {
		return nil
	}
	return &api.AgentAIAgentInterceptor{
		Port: i.Port,
	}
}

func (a *AIAuth) toAPI() *api.AgentAIAuth {
	if a == nil {
		return nil
	}
	return &api.AgentAIAuth{
		Type:   a.Type,
		Header: a.Header,
		Secret: a.Secret.toAPI(),
	}
}

func (s *AISecret) toAPI() *api.AgentAISecret {
	if s == nil {
		return nil
	}
	return &api.AgentAISecret{
		Provider: s.Provider,
		Path:     s.Path,
		Field:    s.Field,
	}
}
