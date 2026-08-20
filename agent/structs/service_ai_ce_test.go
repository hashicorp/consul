// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func TestStructs_ServiceAI_Clone(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		var ai *ServiceAI
		require.Nil(t, ai.Clone())
	})

	originals := map[string]*ServiceAI{
		"inference-model": {
			Role: ServiceAIRoleInferenceModel,
			InferenceModel: &AIInferenceModel{
				Protocol: "openai",
				Path:     "/v1",
				Auth: &AIAuth{
					Type:   "bearer",
					Header: "Authorization",
					Secret: &AISecret{Provider: "vault", Path: "p", Field: "f"},
				},
				Defaults: &AIModelDefaults{MaxTokens: 4096, Temperature: 0.2},
			},
		},
		"mcp-server": {
			Role:      ServiceAIRoleMCPServer,
			MCPServer: &AIMCPServer{Transport: "streamable-http", Path: "/mcp", ProtocolVersion: "2025-03-26"},
		},
		"ai-agent": {
			Role: ServiceAIRoleAgent,
			Agent: &AIAgent{
				Inference: &AIAgentInference{Specialization: []string{"code-review", "coding"}, Vendor: "anthropic"},
				MCP: &AIAgentMCP{
					Port: 15101,
					HITL: &AIAgentMCPHITL{Port: 16101, ApprovalTimeout: "60s"},
				},
				RateLimits:  &AIAgentRateLimits{ToolCallsPerMinute: 120, ToolCallsPerHour: 3000},
				Interceptor: &AIAgentInterceptor{Port: 21101},
			},
		},
	}

	for name, orig := range originals {
		t.Run("deepequal "+name, func(t *testing.T) {
			clone := orig.Clone()
			require.True(t, reflect.DeepEqual(orig, clone), "clone must be deep-equal to original")
			require.NotSame(t, orig, clone)
		})
	}

	t.Run("agent slices are independent", func(t *testing.T) {
		orig := originals["ai-agent"]
		clone := orig.Clone()

		clone.Agent.Inference.Specialization[0] = "MUTATED"
		clone.Agent.MCP.HITL.Port = 99999

		require.Equal(t, "code-review", orig.Agent.Inference.Specialization[0])
		require.Equal(t, 16101, orig.Agent.MCP.HITL.Port)
	})

	t.Run("auth secret pointer is independent", func(t *testing.T) {
		orig := originals["inference-model"]
		clone := orig.Clone()

		clone.InferenceModel.Auth.Secret.Field = "MUTATED"
		require.Equal(t, "f", orig.InferenceModel.Auth.Secret.Field)
	})
}

// TestStructs_ServiceAI_UnmarshalJSON proves the snake_case config/JSON keys and
// the CamelCase API keys parse into the identical struct for every aliased
// multi-word key.
func TestStructs_ServiceAI_UnmarshalJSON(t *testing.T) {
	t.Run("inference_model + nested aliases", func(t *testing.T) {
		snake := []byte(`{
			"role": "inference-model",
			"inference_model": {
				"protocol": "openai",
				"path": "/v1",
				"defaults": { "max_tokens": 2048, "temperature": 0.7 },
				"auth": {
					"type": "bearer",
					"header": "Authorization",
					"secret": { "provider": "vault", "path": "secret/ai", "field": "token" }
				}
			}
		}`)
		camel := []byte(`{
			"Role": "inference-model",
			"InferenceModel": {
				"Protocol": "openai",
				"Path": "/v1",
				"Defaults": { "MaxTokens": 2048, "Temperature": 0.7 },
				"Auth": {
					"Type": "bearer",
					"Header": "Authorization",
					"Secret": { "Provider": "vault", "Path": "secret/ai", "Field": "token" }
				}
			}
		}`)

		var fromSnake, fromCamel ServiceAI
		require.NoError(t, json.Unmarshal(snake, &fromSnake))
		require.NoError(t, json.Unmarshal(camel, &fromCamel))

		require.Equal(t, fromCamel, fromSnake)
		// Assert the snake_case aliases actually mapped (not silently dropped).
		require.NotNil(t, fromSnake.InferenceModel)
		require.Equal(t, 2048, fromSnake.InferenceModel.Defaults.MaxTokens)
		require.Equal(t, "token", fromSnake.InferenceModel.Auth.Secret.Field)
	})

	t.Run("mcp_server + protocol_version aliases", func(t *testing.T) {
		snake := []byte(`{
			"role": "mcp-server",
			"mcp_server": {
				"transport": "streamable-http",
				"path": "/mcp",
				"protocol_version": "2025-03-26"
			}
		}`)
		camel := []byte(`{
			"Role": "mcp-server",
			"MCPServer": {
				"Transport": "streamable-http",
				"Path": "/mcp",
				"ProtocolVersion": "2025-03-26"
			}
		}`)

		var fromSnake, fromCamel ServiceAI
		require.NoError(t, json.Unmarshal(snake, &fromSnake))
		require.NoError(t, json.Unmarshal(camel, &fromCamel))

		require.Equal(t, fromCamel, fromSnake)
		require.NotNil(t, fromSnake.MCPServer)
		require.Equal(t, "2025-03-26", fromSnake.MCPServer.ProtocolVersion)
	})

	t.Run("agent rate_limits + tool_calls aliases", func(t *testing.T) {
		snake := []byte(`{
			"role": "ai-agent",
			"agent": {
				"inference": { "specialization": ["code"], "vendor": "anthropic" },
				"mcp": { "port": 15101, "hitl": { "port": 16101, "approval_timeout": "60s" } },
				"rate_limits": { "tool_calls_per_minute": 120, "tool_calls_per_hour": 3000 },
				"interceptor": { "port": 21101 }
			}
		}`)
		camel := []byte(`{
			"Role": "ai-agent",
			"Agent": {
				"Inference": { "Specialization": ["code"], "Vendor": "anthropic" },
				"MCP": { "Port": 15101, "HITL": { "Port": 16101, "ApprovalTimeout": "60s" } },
				"RateLimits": { "ToolCallsPerMinute": 120, "ToolCallsPerHour": 3000 },
				"Interceptor": { "Port": 21101 }
			}
		}`)

		var fromSnake, fromCamel ServiceAI
		require.NoError(t, json.Unmarshal(snake, &fromSnake))
		require.NoError(t, json.Unmarshal(camel, &fromCamel))

		require.Equal(t, fromCamel, fromSnake)
		require.NotNil(t, fromSnake.Agent.RateLimits)
		require.Equal(t, 120, fromSnake.Agent.RateLimits.ToolCallsPerMinute)
		require.Equal(t, 3000, fromSnake.Agent.RateLimits.ToolCallsPerHour)
		require.NotNil(t, fromSnake.Agent.MCP.HITL)
		require.Equal(t, "60s", fromSnake.Agent.MCP.HITL.ApprovalTimeout)
	})

	t.Run("CamelCase precedence when both present", func(t *testing.T) {
		// When both keys are present the CamelCase value must win (the aux-struct
		// must not overwrite an already-decoded field).
		data := []byte(`{
			"role": "mcp-server",
			"MCPServer": { "transport": "sse", "ProtocolVersion": "camel" },
			"mcp_server": { "transport": "stdio", "protocol_version": "snake" }
		}`)
		var ai ServiceAI
		require.NoError(t, json.Unmarshal(data, &ai))
		require.NotNil(t, ai.MCPServer)
		require.Equal(t, "camel", ai.MCPServer.ProtocolVersion)
	})
}

// multierrorWrapped returns the wrapped errors from a multierror, or a
// single-element slice if it is not a multierror.
func multierrorWrapped(err error) []error {
	type wrappedErrors interface{ WrappedErrors() []error }
	if we, ok := err.(wrappedErrors); ok {
		return we.WrappedErrors()
	}
	return []error{err}
}

// TestStructs_ServiceAI_ToAPI verifies the structs.ServiceAI -> api twin
// converter for all three roles, the nil receiver, and deep-copy isolation.
// Note: api.AgentAISecret carries only provider/path/field — there is no field
// for a resolved secret value, which is itself the no-leak guarantee.
func TestStructs_ServiceAI_ToAPI(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var ai *ServiceAI
		require.Nil(t, ai.ToAPI())
	})

	t.Run("inference-model", func(t *testing.T) {
		in := &ServiceAI{
			Role: ServiceAIRoleInferenceModel,
			InferenceModel: &AIInferenceModel{
				Protocol: "openai",
				Path:     "/v1",
				Defaults: &AIModelDefaults{MaxTokens: 2048, Temperature: 0.7},
				Auth: &AIAuth{
					Type:   "bearer",
					Header: "Authorization",
					Secret: &AISecret{Provider: "vault", Path: "secret/ai", Field: "token"},
				},
			},
		}
		want := &api.AgentServiceAI{
			Role: "inference-model",
			InferenceModel: &api.AgentAIInferenceModel{
				Protocol: "openai",
				Path:     "/v1",
				Defaults: &api.AgentAIModelDefaults{MaxTokens: 2048, Temperature: 0.7},
				Auth: &api.AgentAIAuth{
					Type:   "bearer",
					Header: "Authorization",
					Secret: &api.AgentAISecret{Provider: "vault", Path: "secret/ai", Field: "token"},
				},
			},
		}
		require.Equal(t, want, in.ToAPI())
	})

	t.Run("mcp-server", func(t *testing.T) {
		in := &ServiceAI{
			Role: ServiceAIRoleMCPServer,
			MCPServer: &AIMCPServer{
				Transport:       "sse",
				Path:            "/mcp",
				ProtocolVersion: "2025-03-26",
				Auth: &AIAuth{
					Type:   "bearer",
					Header: "Authorization",
					Secret: &AISecret{Provider: "vault", Path: "secret/mcp", Field: "token"},
				},
			},
		}
		want := &api.AgentServiceAI{
			Role: "mcp-server",
			MCPServer: &api.AgentAIMCPServer{
				Transport:       "sse",
				Path:            "/mcp",
				ProtocolVersion: "2025-03-26",
				Auth: &api.AgentAIAuth{
					Type:   "bearer",
					Header: "Authorization",
					Secret: &api.AgentAISecret{Provider: "vault", Path: "secret/mcp", Field: "token"},
				},
			},
		}
		require.Equal(t, want, in.ToAPI())
	})

	t.Run("ai-agent", func(t *testing.T) {
		in := &ServiceAI{
			Role: ServiceAIRoleAgent,
			Agent: &AIAgent{
				Inference: &AIAgentInference{
					Specialization: []string{"code", "summarize"},
					Vendor:         "openai",
				},
				MCP: &AIAgentMCP{
					Port: 15101,
					HITL: &AIAgentMCPHITL{Port: 16101, ApprovalTimeout: "60s"},
				},
				RateLimits:  &AIAgentRateLimits{ToolCallsPerMinute: 60, ToolCallsPerHour: 1000},
				Interceptor: &AIAgentInterceptor{Port: 21101},
			},
		}
		want := &api.AgentServiceAI{
			Role: "ai-agent",
			Agent: &api.AgentAIAgent{
				Inference: &api.AgentAIAgentInference{
					Specialization: []string{"code", "summarize"},
					Vendor:         "openai",
				},
				MCP: &api.AgentAIAgentMCP{
					Port: 15101,
					HITL: &api.AgentAIAgentMCPHITL{Port: 16101, ApprovalTimeout: "60s"},
				},
				RateLimits:  &api.AgentAIAgentRateLimits{ToolCallsPerMinute: 60, ToolCallsPerHour: 1000},
				Interceptor: &api.AgentAIAgentInterceptor{Port: 21101},
			},
		}
		require.Equal(t, want, in.ToAPI())
	})

	t.Run("deep-copy isolation", func(t *testing.T) {
		in := &ServiceAI{
			Role: ServiceAIRoleAgent,
			Agent: &AIAgent{
				Inference: &AIAgentInference{Specialization: []string{"code"}},
				MCP: &AIAgentMCP{
					Port: 15101,
					HITL: &AIAgentMCPHITL{Port: 16101, ApprovalTimeout: "60s"},
				},
				Interceptor: &AIAgentInterceptor{Port: 21101},
			},
		}
		out := in.ToAPI()

		// Mutating the source must not affect the converted output.
		in.Agent.Inference.Specialization[0] = "MUTATED"
		in.Agent.MCP.HITL.ApprovalTimeout = "MUTATED"
		in.Agent.MCP.Port = 99999

		require.Equal(t, "code", out.Agent.Inference.Specialization[0])
		require.Equal(t, 15101, out.Agent.MCP.Port)
		require.Equal(t, "60s", out.Agent.MCP.HITL.ApprovalTimeout)
	})
}

// TestStructs_NodeService_Validate_AI_CE_NotSupported verifies that the CE
// edition rejects any service definition that carries an AI block, regardless
// of whether the AI sub-fields are themselves valid.
func TestStructs_NodeService_Validate_AI_CE_NotSupported(t *testing.T) {
	cases := []struct {
		name string
		ai   *ServiceAI
	}{
		{
			name: "inference-model role",
			ai: &ServiceAI{
				Role:           ServiceAIRoleInferenceModel,
				InferenceModel: &AIInferenceModel{Protocol: aiProtocolOpenAI},
			},
		},
		{
			name: "mcp-server role",
			ai: &ServiceAI{
				Role: ServiceAIRoleMCPServer,
				MCPServer: &AIMCPServer{
					Transport:       aiTransportStreamableHTTP,
					ProtocolVersion: "2025-03-26",
				},
			},
		},
		{
			name: "ai-agent role",
			ai: &ServiceAI{
				Role:  ServiceAIRoleAgent,
				Agent: &AIAgent{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sd := &ServiceDefinition{
				Name: "ai-svc",
				Port: 8080,
				AI:   tc.ai,
			}
			err := sd.Validate()
			require.Error(t, err, "expected CE to reject a service with an AI block")
			require.True(t, strings.Contains(err.Error(), "ai is ent only feature"),
				"expected CE restriction message, got: %s", err.Error())
		})
	}
}
