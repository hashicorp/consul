// Copyright (c) HashiCorp, Inc.

package api

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntries_JWTProvider(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	entries := c.ConfigEntries()

	testutil.RunStep(t, "set and get", func(t *testing.T) {
		connectTimeout := time.Duration(5) * time.Second
		jwtProvider := &JWTProviderConfigEntry{
			Name: "okta",
			Kind: JWTProvider,
			JSONWebKeySet: &JSONWebKeySet{
				Remote: &RemoteJWKS{
					FetchAsynchronously: true,
					URI:                 "https://example.com/.well-known/jwks.json",
					JWKSCluster: &JWKSCluster{
						DiscoveryType:  "STATIC",
						ConnectTimeout: connectTimeout,
						TLSCertificates: &JWKSTLSCertificate{
							TrustedCA: &JWKSTLSCertTrustedCA{
								Filename: "myfile.cert",
							},
						},
					},
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
