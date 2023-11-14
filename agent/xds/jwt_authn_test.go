// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_http_jwt_authn_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type ixnOpts struct {
	src    string
	action structs.IntentionAction
	jwt    *structs.IntentionJWTRequirement
	perms  *structs.IntentionPermission
}

func makeProvider(name string) structs.IntentionJWTProvider {
	return structs.IntentionJWTProvider{
		Name: name,
	}
}

func decodeJWKS(t *testing.T, jw string) string {
	s, err := base64.StdEncoding.DecodeString(jw)
	require.NoError(t, err)
	return string(s)
}

var (
	token         = "eyJrZXlzIjogW3sKICAiY3J2IjogIlAtMjU2IiwKICAia2V5X29wcyI6IFsKICAgICJ2ZXJpZnkiCiAgXSwKICAia3R5IjogIkVDIiwKICAieCI6ICJXYzl1WnVQYUI3S2gyRk1jOXd0SmpSZThYRDR5VDJBWU5BQWtyWWJWanV3IiwKICAieSI6ICI2OGhSVEppSk5Pd3RyaDRFb1BYZVZuUnVIN2hpU0RKX2xtYmJqZkRmV3EwIiwKICAiYWxnIjogIkVTMjU2IiwKICAidXNlIjogInNpZyIsCiAgImtpZCI6ICJhYzFlOGY5MGVkZGY2MWM0MjljNjFjYTA1YjRmMmUwNyIKfV19"
	oktaProvider  = makeProvider("okta")
	auth0Provider = makeProvider("auth0")
	fakeProvider  = makeProvider("fake-provider")
	oktaIntention = &structs.IntentionJWTRequirement{
		Providers: []*structs.IntentionJWTProvider{
			&oktaProvider,
		},
	}
	auth0Intention = &structs.IntentionJWTRequirement{
		Providers: []*structs.IntentionJWTProvider{
			&auth0Provider,
		},
	}
	fakeIntention = &structs.IntentionJWTRequirement{
		Providers: []*structs.IntentionJWTProvider{
			&fakeProvider,
		},
	}
	multiProviderIntentions = &structs.IntentionJWTRequirement{
		Providers: []*structs.IntentionJWTProvider{
			&oktaProvider,
			&auth0Provider,
		},
	}
	pWithOktaProvider = &structs.IntentionPermission{
		Action: structs.IntentionActionAllow,
		HTTP: &structs.IntentionHTTPPermission{
			PathPrefix: "some-special-path",
		},
		JWT: oktaIntention,
	}
	pWithMultiProviders = &structs.IntentionPermission{
		Action: structs.IntentionActionAllow,
		HTTP: &structs.IntentionHTTPPermission{
			PathPrefix: "some-special-path",
		},
		JWT: multiProviderIntentions,
	}
	pWithNoJWT = &structs.IntentionPermission{
		Action: structs.IntentionActionAllow,
		HTTP: &structs.IntentionHTTPPermission{
			PathPrefix: "some-special-path",
		},
	}
	fullRetryPolicy = &structs.JWKSRetryPolicy{
		RetryPolicyBackOff: &structs.RetryPolicyBackOff{
			BaseInterval: 0,
			MaxInterval:  10,
		},
		NumRetries: 1,
	}
	oktaRemoteJWKS = &structs.RemoteJWKS{
		RequestTimeoutMs:    1000,
		FetchAsynchronously: true,
		URI:                 "https://example-okta.com/.well-known/jwks.json",
	}
	auth0RemoteJWKS = &structs.RemoteJWKS{
		RequestTimeoutMs:    1000,
		FetchAsynchronously: true,
		URI:                 "https://example-auth0.com/.well-known/jwks.json",
	}
	extendedRemoteJWKS = &structs.RemoteJWKS{
		RequestTimeoutMs:    1000,
		FetchAsynchronously: true,
		URI:                 "https://example-okta.com/.well-known/jwks.json",
		RetryPolicy:         fullRetryPolicy,
		CacheDuration:       20,
	}
	localJWKS = &structs.LocalJWKS{
		JWKS: token,
	}
	localJWKSFilename = &structs.LocalJWKS{
		Filename: "file.txt",
	}
)

func makeTestIntention(t *testing.T, opts ixnOpts) *structs.Intention {
	t.Helper()
	ixn := structs.TestIntention(t)
	ixn.SourceName = opts.src

	if opts.jwt != nil {
		ixn.JWT = opts.jwt
	}
	if opts.perms != nil {
		ixn.Permissions = append(ixn.Permissions, opts.perms)
	}

	if opts.action != "" {
		ixn.Action = opts.action
	}
	return ixn
}

func TestMakeJWTAUTHFilters(t *testing.T) {
	simplified := func(ixns ...*structs.Intention) structs.SimplifiedIntentions {
		return structs.SimplifiedIntentions(ixns)
	}

	var (
		remoteCE = map[string]*structs.JWTProviderConfigEntry{
			"okta": {
				Kind:   "jwt-provider",
				Name:   "okta",
				Issuer: "test-issuer",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Remote: oktaRemoteJWKS,
				},
			},
			"auth0": {
				Kind:   "jwt-provider",
				Name:   "auth0",
				Issuer: "another-issuer",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Remote: auth0RemoteJWKS,
				},
			},
		}
		localCE = map[string]*structs.JWTProviderConfigEntry{
			"okta": {
				Kind:   "jwt-provider",
				Name:   "okta",
				Issuer: "test-issuer",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Local: localJWKS,
				},
			},
		}
	)

	// All tests here depend on golden files located under: agent/xds/testdata/jwt_authn/*
	tests := map[string]struct {
		intentions structs.SimplifiedIntentions
		provider   map[string]*structs.JWTProviderConfigEntry
	}{
		"no-provider": {
			intentions: simplified(makeTestIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow})),
			provider:   nil,
		},
		"remote-provider": {
			intentions: simplified(makeTestIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: oktaIntention})),
			provider:   remoteCE,
		},
		"local-provider": {
			intentions: simplified(makeTestIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: oktaIntention})),
			provider:   localCE,
		},
		"intention-with-path": {
			intentions: simplified(makeTestIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, perms: pWithOktaProvider})),
			provider:   remoteCE,
		},
		"top-level-provider-with-permission": {
			intentions: simplified(makeTestIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: oktaIntention, perms: pWithOktaProvider})),
			provider:   remoteCE,
		},
		"multiple-providers-and-one-permission": {
			intentions: simplified(makeTestIntention(t, ixnOpts{src: "web", action: structs.IntentionActionAllow, jwt: multiProviderIntentions, perms: pWithOktaProvider})),
			provider:   remoteCE,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			filter, err := makeJWTAuthFilter(tt.provider, tt.intentions)
			require.NoError(t, err)
			gotJSON := protoToJSON(t, filter)
			require.JSONEq(t, goldenSimple(t, filepath.Join("jwt_authn", name), gotJSON), gotJSON)
		})
	}
}

func TestCollectJWTProviders(t *testing.T) {
	tests := map[string]struct {
		intention *structs.Intention
		expected  []*structs.IntentionJWTProvider
	}{
		"empty-top-level-jwt-and-empty-permissions": {
			intention: makeTestIntention(t, ixnOpts{src: "web"}),
			expected:  []*structs.IntentionJWTProvider{},
		},
		"top-level-jwt-and-empty-permissions": {
			intention: makeTestIntention(t, ixnOpts{src: "web", jwt: oktaIntention}),
			expected:  []*structs.IntentionJWTProvider{&oktaProvider},
		},
		"multi-top-level-jwt-and-empty-permissions": {
			intention: makeTestIntention(t, ixnOpts{src: "web", jwt: multiProviderIntentions}),
			expected:  []*structs.IntentionJWTProvider{&oktaProvider, &auth0Provider},
		},
		"top-level-jwt-and-one-jwt-permission": {
			intention: makeTestIntention(t, ixnOpts{src: "web", jwt: auth0Intention, perms: pWithOktaProvider}),
			expected:  []*structs.IntentionJWTProvider{&auth0Provider, &oktaProvider},
		},
		"top-level-jwt-and-multi-jwt-permissions": {
			intention: makeTestIntention(t, ixnOpts{src: "web", jwt: fakeIntention, perms: pWithMultiProviders}),
			expected:  []*structs.IntentionJWTProvider{&fakeProvider, &oktaProvider, &auth0Provider},
		},
		"empty-top-level-jwt-and-one-jwt-permission": {
			intention: makeTestIntention(t, ixnOpts{src: "web", perms: pWithOktaProvider}),
			expected:  []*structs.IntentionJWTProvider{&oktaProvider},
		},
		"empty-top-level-jwt-and-multi-jwt-permission": {
			intention: makeTestIntention(t, ixnOpts{src: "web", perms: pWithMultiProviders}),
			expected:  []*structs.IntentionJWTProvider{&oktaProvider, &auth0Provider},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			reqs := collectJWTProviders(tt.intention)
			require.ElementsMatch(t, reqs, tt.expected)
		})
	}
}

func TestGetPermissionsProviders(t *testing.T) {
	tests := map[string]struct {
		perms    []*structs.IntentionPermission
		expected []*structs.IntentionJWTProvider
	}{
		"empty-permissions": {
			perms:    []*structs.IntentionPermission{},
			expected: []*structs.IntentionJWTProvider{},
		},
		"nil-permissions": {
			perms:    nil,
			expected: []*structs.IntentionJWTProvider{},
		},
		"permissions-with-no-jwt": {
			perms:    []*structs.IntentionPermission{pWithNoJWT},
			expected: []*structs.IntentionJWTProvider{},
		},
		"permissions-with-one-jwt": {
			perms:    []*structs.IntentionPermission{pWithOktaProvider, pWithNoJWT},
			expected: []*structs.IntentionJWTProvider{&oktaProvider},
		},
		"permissions-with-multiple-jwt": {
			perms:    []*structs.IntentionPermission{pWithMultiProviders, pWithNoJWT},
			expected: []*structs.IntentionJWTProvider{&auth0Provider, &oktaProvider},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			p := getPermissionsProviders(tt.perms)
			require.ElementsMatch(t, p, tt.expected)
		})
	}
}

func TestBuildJWTProviderConfig(t *testing.T) {
	var (
		fullCE = structs.JWTProviderConfigEntry{
			Kind:      "jwt-provider",
			Issuer:    "auth0",
			Audiences: []string{"aud"},
			JSONWebKeySet: &structs.JSONWebKeySet{
				Local: &structs.LocalJWKS{
					JWKS: token,
				},
			},
			Forwarding: &structs.JWTForwardingConfig{HeaderName: "user-token"},
			Locations: []*structs.JWTLocation{
				{Header: &structs.JWTLocationHeader{Forward: true, Name: "Authorization", ValuePrefix: "Bearer"}},
			},
		}
		ceRemoteJWKS = structs.JWTProviderConfigEntry{
			Kind:      "jwt-provider",
			Issuer:    "auth0",
			Audiences: []string{"aud"},
			JSONWebKeySet: &structs.JSONWebKeySet{
				Remote: oktaRemoteJWKS,
			},
		}
		ceInvalidJWKS = structs.JWTProviderConfigEntry{JSONWebKeySet: &structs.JSONWebKeySet{}}
	)

	tests := map[string]struct {
		ce            *structs.JWTProviderConfigEntry
		expected      *envoy_http_jwt_authn_v3.JwtProvider
		expectedError string
		providerName  string
	}{
		"config-entry-with-invalid-localJWKS": {
			ce:            &ceInvalidJWKS,
			expectedError: "invalid jwt provider config; missing JSONWebKeySet for provider",
		},
		"valid-config-entry": {
			ce: &fullCE,
			expected: &envoy_http_jwt_authn_v3.JwtProvider{
				Issuer:                  fullCE.Issuer,
				Audiences:               fullCE.Audiences,
				ForwardPayloadHeader:    "user-token",
				PayloadInMetadata:       buildPayloadInMetadataKey(ceRemoteJWKS.Name),
				PadForwardPayloadHeader: false,
				Forward:                 true,
				JwksSourceSpecifier: &envoy_http_jwt_authn_v3.JwtProvider_LocalJwks{
					LocalJwks: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: decodeJWKS(t, localJWKS.JWKS),
						},
					},
				},
				FromHeaders: []*envoy_http_jwt_authn_v3.JwtHeader{{Name: "Authorization", ValuePrefix: "Bearer"}},
			},
		},
		"entry-with-remote-jwks": {
			ce: &ceRemoteJWKS,
			expected: &envoy_http_jwt_authn_v3.JwtProvider{
				Issuer:            fullCE.Issuer,
				Audiences:         fullCE.Audiences,
				PayloadInMetadata: buildPayloadInMetadataKey(ceRemoteJWKS.Name),
				JwksSourceSpecifier: &envoy_http_jwt_authn_v3.JwtProvider_RemoteJwks{
					RemoteJwks: &envoy_http_jwt_authn_v3.RemoteJwks{
						HttpUri: &envoy_core_v3.HttpUri{
							Uri:              oktaRemoteJWKS.URI,
							HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{Cluster: makeJWKSClusterName(ceRemoteJWKS.Name)},
							Timeout:          &durationpb.Duration{Seconds: 1},
						},
						AsyncFetch: &envoy_http_jwt_authn_v3.JwksAsyncFetch{
							FastListener: oktaRemoteJWKS.FetchAsynchronously,
						},
					},
				},
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			res, err := buildJWTProviderConfig(tt.ce)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.Equal(t, res, tt.expected)
			}
		})
	}

}

func TestMakeLocalJWKS(t *testing.T) {
	tests := map[string]struct {
		jwks          *structs.LocalJWKS
		providerName  string
		expected      *envoy_http_jwt_authn_v3.JwtProvider_LocalJwks
		expectedError string
	}{
		"invalid-base64-jwks": {
			jwks:          &structs.LocalJWKS{JWKS: "decoded-jwks"},
			expectedError: "illegal base64 data",
		},
		"no-jwks-and-no-filename": {
			jwks:          &structs.LocalJWKS{},
			expectedError: "invalid jwt provider config; missing JWKS/Filename for local provider",
		},
		"localjwks-with-filename": {
			jwks: localJWKSFilename,
			expected: &envoy_http_jwt_authn_v3.JwtProvider_LocalJwks{
				LocalJwks: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: localJWKSFilename.Filename,
					},
				},
			},
		},
		"localjwks-with-jwks": {
			jwks: localJWKS,
			expected: &envoy_http_jwt_authn_v3.JwtProvider_LocalJwks{
				LocalJwks: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: decodeJWKS(t, localJWKS.JWKS),
					},
				},
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			res, err := makeLocalJWKS(tt.jwks, tt.providerName)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.Equal(t, res, tt.expected)
			}
		})
	}
}

func TestMakeRemoteJWKS(t *testing.T) {
	tests := map[string]struct {
		jwks         *structs.RemoteJWKS
		providerName string
		expected     *envoy_http_jwt_authn_v3.JwtProvider_RemoteJwks
	}{
		"with-no-cache-duration": {
			jwks:         oktaRemoteJWKS,
			providerName: "auth0",
			expected: &envoy_http_jwt_authn_v3.JwtProvider_RemoteJwks{
				RemoteJwks: &envoy_http_jwt_authn_v3.RemoteJwks{
					HttpUri: &envoy_core_v3.HttpUri{
						Uri:              oktaRemoteJWKS.URI,
						HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{Cluster: makeJWKSClusterName("auth0")},
						Timeout:          &durationpb.Duration{Seconds: 1},
					},
					AsyncFetch: &envoy_http_jwt_authn_v3.JwksAsyncFetch{
						FastListener: oktaRemoteJWKS.FetchAsynchronously,
					},
				},
			},
		},
		"with-retry-policy": {
			jwks:         extendedRemoteJWKS,
			providerName: "okta",
			expected: &envoy_http_jwt_authn_v3.JwtProvider_RemoteJwks{
				RemoteJwks: &envoy_http_jwt_authn_v3.RemoteJwks{
					HttpUri: &envoy_core_v3.HttpUri{
						Uri:              oktaRemoteJWKS.URI,
						HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{Cluster: makeJWKSClusterName("okta")},
						Timeout:          &durationpb.Duration{Seconds: 1},
					},
					AsyncFetch: &envoy_http_jwt_authn_v3.JwksAsyncFetch{
						FastListener: oktaRemoteJWKS.FetchAsynchronously,
					},
					RetryPolicy:   buildJWTRetryPolicy(extendedRemoteJWKS.RetryPolicy),
					CacheDuration: &durationpb.Duration{Seconds: int64(extendedRemoteJWKS.CacheDuration)},
				},
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			res := makeRemoteJWKS(tt.jwks, tt.providerName)
			require.Equal(t, res, tt.expected)
		})
	}
}

func TestBuildJWTRetryPolicy(t *testing.T) {
	var (
		noBackofRetryPolicy = &structs.JWKSRetryPolicy{NumRetries: 1}
		noNumRetriesPolicy  = &structs.JWKSRetryPolicy{
			RetryPolicyBackOff: &structs.RetryPolicyBackOff{BaseInterval: 0, MaxInterval: 10},
		}
	)
	tests := map[string]struct {
		retryPolicy *structs.JWKSRetryPolicy
		expected    *envoy_core_v3.RetryPolicy
	}{
		"nil-retry-policy": {
			retryPolicy: nil,
			expected:    nil,
		},
		"retry-policy-with-no-backoff": {
			retryPolicy: noBackofRetryPolicy,
			expected:    &envoy_core_v3.RetryPolicy{NumRetries: wrapperspb.UInt32(uint32(1))},
		},
		"retry-policy-with-backoff": {
			retryPolicy: fullRetryPolicy,
			expected: &envoy_core_v3.RetryPolicy{
				NumRetries: wrapperspb.UInt32(uint32(1)),
				RetryBackOff: &envoy_core_v3.BackoffStrategy{
					BaseInterval: structs.DurationToProto(0),
					MaxInterval:  structs.DurationToProto(10),
				},
			},
		},
		"retry-policy-with-no-retries": {
			retryPolicy: noNumRetriesPolicy,
			expected: &envoy_core_v3.RetryPolicy{
				RetryBackOff: &envoy_core_v3.BackoffStrategy{
					BaseInterval: structs.DurationToProto(0),
					MaxInterval:  structs.DurationToProto(10),
				},
				NumRetries: wrapperspb.UInt32(uint32(0)),
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			res := buildJWTRetryPolicy(tt.retryPolicy)

			require.Equal(t, res, tt.expected)
		})
	}
}

func TestHasJWTconfig(t *testing.T) {
	tests := map[string]struct {
		perms    []*structs.IntentionPermission
		expected bool
	}{
		"empty-permissions": {
			perms:    []*structs.IntentionPermission{},
			expected: false,
		},
		"nil-permissions": {
			perms:    nil,
			expected: false,
		},
		"permissions-with-no-jwt": {
			perms:    []*structs.IntentionPermission{pWithNoJWT},
			expected: false,
		},
		"permissions-with-one-jwt": {
			perms:    []*structs.IntentionPermission{pWithOktaProvider, pWithNoJWT},
			expected: true,
		},
		"permissions-with-multiple-jwt": {
			perms:    []*structs.IntentionPermission{pWithMultiProviders, pWithNoJWT},
			expected: true,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			res := hasJWTconfig(tt.perms)
			require.Equal(t, res, tt.expected)
		})
	}
}
