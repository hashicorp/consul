// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func serviceDefaultsWith(name string, ext api.EnvoyExtension) *api.ServiceConfigEntry {
	return &api.ServiceConfigEntry{
		Kind:            api.ServiceDefaults,
		Name:            name,
		Protocol:        "http",
		EnvoyExtensions: []api.EnvoyExtension{ext},
	}
}

// TestEnvoyExtensions_ConfigEntryValidation verifies that servers validate
// Envoy extension arguments when the config entry is written, so a mistake is
// reported to the operator instead of failing later inside every proxy.
func TestEnvoyExtensions_ConfigEntryValidation(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	entries := e.server.ConfigEntries()

	const script = "function envoy_on_request(request_handle) end"

	t.Run("valid extensions are stored with their arguments and flags", func(t *testing.T) {
		valid := map[string]api.EnvoyExtension{
			"ext-lua": {
				Name:     "builtin/lua",
				Required: true,
				Arguments: map[string]any{
					"ProxyType": "connect-proxy", "Listener": "inbound", "Script": script,
				},
			},
			"ext-lambda": {
				Name: "builtin/aws/lambda",
				Arguments: map[string]any{
					"ProxyType": "connect-proxy",
					"ARN":       "arn:aws:lambda:us-east-1:111111111111:function:f1",
				},
			},
			"ext-authz": {
				Name: "builtin/ext-authz",
				Arguments: map[string]any{
					"ProxyType": "connect-proxy",
					"Config": map[string]any{
						"GrpcService": map[string]any{"Target": map[string]any{"Service": map[string]any{"Name": "authz"}}},
					},
				},
			},
			"ext-override": {
				Name: "builtin/property-override",
				Arguments: map[string]any{
					"ProxyType": "connect-proxy",
					"Patches": []any{map[string]any{
						"ResourceFilter": map[string]any{"ResourceType": "cluster", "TrafficDirection": "outbound"},
						"Op":             "add",
						"Path":           "/respect_dns_ttl",
						"Value":          true,
					}},
				},
			},
		}

		for svc, ext := range valid {
			setEntry(t, e.server, serviceDefaultsWith(svc, ext))

			got, _, err := entries.Get(api.ServiceDefaults, svc, nil)
			require.NoError(t, err, svc)
			stored := got.(*api.ServiceConfigEntry).EnvoyExtensions
			require.Len(t, stored, 1, svc)
			require.Equal(t, ext.Name, stored[0].Name)
			require.Equal(t, ext.Required, stored[0].Required, svc)
			require.Equal(t, "connect-proxy", stored[0].Arguments["ProxyType"], svc)
		}
	})

	t.Run("extensions can be set globally in proxy-defaults", func(t *testing.T) {
		setEntry(t, e.server, &api.ProxyConfigEntry{
			Kind: api.ProxyDefaults,
			Name: api.ProxyConfigGlobal,
			EnvoyExtensions: []api.EnvoyExtension{{
				Name:      "builtin/lua",
				Arguments: map[string]any{"ProxyType": "connect-proxy", "Listener": "outbound", "Script": script},
			}},
		})

		got, _, err := entries.Get(api.ProxyDefaults, api.ProxyConfigGlobal, nil)
		require.NoError(t, err)
		require.Len(t, got.(*api.ProxyConfigEntry).EnvoyExtensions, 1)

		_, err = entries.Delete(api.ProxyDefaults, api.ProxyConfigGlobal, nil)
		require.NoError(t, err)
	})

	for name, tc := range map[string]struct {
		ext     api.EnvoyExtension
		wantErr string
	}{
		"unknown extension": {
			ext:     api.EnvoyExtension{Name: "builtin/does-not-exist"},
			wantErr: "is not a built-in extension",
		},
		"lambda requires an ARN": {
			ext:     api.EnvoyExtension{Name: "builtin/aws/lambda", Arguments: map[string]any{"ProxyType": "connect-proxy"}},
			wantErr: "ARN is required",
		},
		"lua requires a script": {
			ext:     api.EnvoyExtension{Name: "builtin/lua", Arguments: map[string]any{"ProxyType": "connect-proxy", "Listener": "inbound"}},
			wantErr: "missing Script",
		},
		"lua rejects an unknown proxy type": {
			ext:     api.EnvoyExtension{Name: "builtin/lua", Arguments: map[string]any{"ProxyType": "bogus", "Listener": "inbound", "Script": script}},
			wantErr: "unexpected ProxyType",
		},
		"ext-authz requires a service definition": {
			ext:     api.EnvoyExtension{Name: "builtin/ext-authz", Arguments: map[string]any{"ProxyType": "connect-proxy"}},
			wantErr: "exactly one of GrpcService or HttpService",
		},
	} {
		tc := tc
		t.Run("rejected: "+name, func(t *testing.T) {
			svc := "rejected-" + tc.ext.Name
			ok, _, err := entries.Set(serviceDefaultsWith(svc, tc.ext), nil)
			require.Error(t, err)
			require.False(t, ok)
			require.Contains(t, err.Error(), tc.wantErr)

			_, _, err = entries.Get(api.ServiceDefaults, svc, nil)
			require.Error(t, err, "a rejected entry must not be stored")
		})
	}
}
