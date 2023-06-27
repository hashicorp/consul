// Copyright (c) HashiCorp, Inc.

package api

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestAPI_ConfigEntries_JWTProvider(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	entries := c.ConfigEntries()

	testutil.RunStep(t, "set and get", func(t *testing.T) {
		jwtProvider := &JWTProviderConfigEntry{
			Name: "okta",
			Kind: JWTProvider,
			JSONWebKeySet: &JSONWebKeySet{
				Local: &LocalJWKS{
					Filename: "test.txt",
				},
			},
			Meta: map[string]string{
				"gir": "zim",
			},
		}

		_, wm, err := entries.Set(jwtProvider, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		entry, qm, err := entries.Get(JWTProvider, "okta", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		result, ok := entry.(*JWTProviderConfigEntry)
		require.True(t, ok)

		require.Equal(t, jwtProvider.Name, result.Name)
		require.Equal(t, jwtProvider.JSONWebKeySet, result.JSONWebKeySet)
	})
}
