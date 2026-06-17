// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtProcServiceName(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args map[string]any
		want string
	}{
		"nil args": {
			args: nil,
			want: "",
		},
		"grpc service target (PascalCase)": {
			args: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "processor"}},
					},
				},
			},
			want: "processor",
		},
		"http service target (camelCase)": {
			args: map[string]any{
				"config": map[string]any{
					"httpService": map[string]any{
						"target": map[string]any{"service": map[string]any{"name": "decider"}},
					},
				},
			},
			want: "decider",
		},
		"grpc preferred over http": {
			args: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "grpc-svc"}},
					},
					"HttpService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "http-svc"}},
					},
				},
			},
			want: "grpc-svc",
		},
		"uri target yields empty name": {
			args: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
			want: "",
		},
		"no service configured": {
			args: map[string]any{"Config": map[string]any{}},
			want: "",
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.want, extProcServiceName(c.args))
		})
	}
}

func TestExtProcChainProtocol(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args map[string]any
		want string
	}{
		"grpc service yields grpc": {
			args: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "processor"}},
					},
				},
			},
			want: "grpc",
		},
		"http service yields http": {
			args: map[string]any{
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "processor"}},
					},
				},
			},
			want: "http",
		},
		"no service defaults to http": {
			args: map[string]any{"Config": map[string]any{}},
			want: "http",
		},
		"nil args defaults to http": {
			args: nil,
			want: "http",
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.want, extProcChainProtocol(c.args))
		})
	}
}

func TestParseExtProcArgs_Service(t *testing.T) {
	t.Parallel()

	t.Run("returns grpc service", func(t *testing.T) {
		parsed := parseExtProcArgs(map[string]any{
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target": map[string]any{"Service": map[string]any{"Name": "grpc-svc"}},
				},
			},
		})
		svc := parsed.service()
		require.NotNil(t, svc)
		require.Equal(t, "grpc-svc", svc.Target.Service.Name)
	})

	t.Run("returns http service", func(t *testing.T) {
		parsed := parseExtProcArgs(map[string]any{
			"Config": map[string]any{
				"HttpService": map[string]any{
					"Target": map[string]any{"Service": map[string]any{"Name": "http-svc"}},
				},
			},
		})
		svc := parsed.service()
		require.NotNil(t, svc)
		require.Equal(t, "http-svc", svc.Target.Service.Name)
	})

	t.Run("nil when no service configured", func(t *testing.T) {
		parsed := parseExtProcArgs(map[string]any{"Config": map[string]any{}})
		require.Nil(t, parsed.service())
	})
}
