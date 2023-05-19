// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestMakeJWTAUTHFilters(t *testing.T) {
	type ixnOpts struct {
		src    string
		action structs.IntentionAction
		jwt    *structs.IntentionJWTRequirement
		perms  *structs.IntentionPermission
	}

	testIntention := func(t *testing.T, opts ixnOpts) *structs.Intention {
		t.Helper()
		ixn := structs.TestIntention(t)
		ixn.SourceName = opts.src

		if opts.jwt != nil {
			ixn.JWT = opts.jwt
		}
		if opts.perms != nil {
			ixn.Permissions = append(ixn.Permissions, opts.perms)
		} else {
			ixn.Action = opts.action
		}
		return ixn
	}

	simplified := func(ixns ...*structs.Intention) structs.SimplifiedIntentions {
		return structs.SimplifiedIntentions(ixns)
	}

	var (
		oktaProvider = structs.IntentionJWTProvider{
			Name: "okta",
		}
		auth0Provider = structs.IntentionJWTProvider{
			Name: "auth0",
		}
		singleProviderIntention = &structs.IntentionJWTRequirement{
			Providers: []*structs.IntentionJWTProvider{
				&oktaProvider,
			},
		}
		multiProviderIntentions = &structs.IntentionJWTRequirement{
			Providers: []*structs.IntentionJWTProvider{
				&oktaProvider,
				&auth0Provider,
			},
		}
		remoteCE = map[string]*structs.JWTProviderConfigEntry{
			"okta": {
				Kind:   "jwt-provider",
				Name:   "okta",
				Issuer: "test-issuer",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Remote: &structs.RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
					},
				},
			},
			"auth0": {
				Kind:   "jwt-provider",
				Name:   "auth0",
				Issuer: "another-issuer",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Remote: &structs.RemoteJWKS{
						FetchAsynchronously: true,
						URI:                 "https://example.com/.well-known/jwks.json",
					},
				},
			},
		}
		localCE = map[string]*structs.JWTProviderConfigEntry{
			"okta": {
				Kind:   "jwt-provider",
				Name:   "okta",
				Issuer: "test-issuer",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Local: &structs.LocalJWKS{
						JWKS: "eyJrZXlzIjogW3sKICAiY3J2IjogIlAtMjU2IiwKICAia2V5X29wcyI6IFsKICAgICJ2ZXJpZnkiCiAgXSwKICAia3R5IjogIkVDIiwKICAieCI6ICJXYzl1WnVQYUI3S2gyRk1jOXd0SmpSZThYRDR5VDJBWU5BQWtyWWJWanV3IiwKICAieSI6ICI2OGhSVEppSk5Pd3RyaDRFb1BYZVZuUnVIN2hpU0RKX2xtYmJqZkRmV3EwIiwKICAiYWxnIjogIkVTMjU2IiwKICAidXNlIjogInNpZyIsCiAgImtpZCI6ICJhYzFlOGY5MGVkZGY2MWM0MjljNjFjYTA1YjRmMmUwNyIKfV19",
					},
				},
			},
		}
		pWithOneProvider = &structs.IntentionPermission{
			Action: structs.IntentionActionAllow,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/some-special-path",
			},
			JWT: singleProviderIntention,
		}
		permWithPath = &structs.IntentionPermission{
			Action: structs.IntentionActionAllow,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/some-special-path",
			},
			JWT: singleProviderIntention,
		}
	)

	tests := map[string]struct {
		intentions structs.SimplifiedIntentions
		provider   map[string]*structs.JWTProviderConfigEntry
	}{
		"remote-provider": {
			intentions: simplified(testIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: singleProviderIntention})),
			provider:   remoteCE,
		},
		"local-provider": {
			intentions: simplified(testIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: singleProviderIntention})),
			provider:   localCE,
		},
		"intention-with-path": {
			intentions: simplified(testIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, perms: pWithOneProvider})),
			provider:   remoteCE,
		},
		"top-level-provider-with-permission": {
			intentions: simplified(testIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: singleProviderIntention, perms: permWithPath})),
			provider:   remoteCE,
		},
		"multiple-providers-and-permissions": {
			intentions: simplified(testIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: multiProviderIntentions, perms: permWithPath})),
			provider:   remoteCE,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Run("jwt filter", func(t *testing.T) {
				filter, err := makeJWTAuthFilter(tt.provider, tt.intentions)
				require.NoError(t, err)

				t.Run("current", func(t *testing.T) {
					gotJSON := protoToJSON(t, filter)

					require.JSONEq(t, goldenSimple(t, filepath.Join("jwt_authn", name), gotJSON), gotJSON)
				})
			})
		})
	}
}
