// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"encoding/base64"
	"fmt"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_jwt_authn_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/hashicorp/consul/agent/structs"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	jwtEnvoyFilter       = "envoy.filters.http.jwt_authn"
	jwtMetadataKeyPrefix = "jwt_payload"
	jwksClusterPrefix    = "jwks_cluster"
)

// makeJWTAuthFilter builds jwt filter for envoy. It limits its use to referenced provider rather than every provider.
//
// Eg. If you have three providers: okta, auth0 and fusionAuth and only okta is referenced in your intentions, then this
// will create a jwt-auth filter containing just okta in the list of providers.
func makeJWTAuthFilter(providerMap map[string]*structs.JWTProviderConfigEntry, intentions structs.SimplifiedIntentions) (*envoy_http_v3.HttpFilter, error) {
	providers := map[string]*envoy_http_jwt_authn_v3.JwtProvider{}
	var jwtRequirements []*envoy_http_jwt_authn_v3.JwtRequirement

	for _, intention := range intentions {
		if intention.JWT == nil && !hasJWTconfig(intention.Permissions) {
			continue
		}
		for _, p := range collectJWTProviders(intention) {
			providerName := p.Name
			if _, ok := providers[providerName]; ok {
				continue
			}

			providerCE, ok := providerMap[providerName]
			if !ok {
				return nil, fmt.Errorf("provider specified in intention does not exist. Provider name: %s", providerName)
			}

			envoyCfg, err := buildJWTProviderConfig(providerCE)
			if err != nil {
				return nil, err
			}
			providers[providerName] = envoyCfg
			reqs := providerToJWTRequirement(providerCE)
			jwtRequirements = append(jwtRequirements, reqs)
		}
	}

	if len(jwtRequirements) == 0 {
		//do not add jwt_authn filter when intentions don't have JWTs
		return nil, nil
	}

	cfg := &envoy_http_jwt_authn_v3.JwtAuthentication{
		Providers: providers,
		Rules: []*envoy_http_jwt_authn_v3.RequirementRule{
			{
				Match: &envoy_route_v3.RouteMatch{
					PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/"},
				},
				RequirementType: makeJWTRequirementRule(andJWTRequirements(jwtRequirements)),
			},
		},
	}
	return makeEnvoyHTTPFilter(jwtEnvoyFilter, cfg)
}

func makeJWTRequirementRule(r *envoy_http_jwt_authn_v3.JwtRequirement) *envoy_http_jwt_authn_v3.RequirementRule_Requires {
	return &envoy_http_jwt_authn_v3.RequirementRule_Requires{
		Requires: r,
	}
}

// andJWTRequirements combines list of jwt requirements into a single jwt requirement.
func andJWTRequirements(reqs []*envoy_http_jwt_authn_v3.JwtRequirement) *envoy_http_jwt_authn_v3.JwtRequirement {
	switch len(reqs) {
	case 0:
		return nil
	case 1:
		return reqs[0]
	default:
		return &envoy_http_jwt_authn_v3.JwtRequirement{
			RequiresType: &envoy_http_jwt_authn_v3.JwtRequirement_RequiresAll{
				RequiresAll: &envoy_http_jwt_authn_v3.JwtRequirementAndList{
					Requirements: reqs,
				},
			},
		}
	}
}

// providerToJWTRequirement builds the envoy jwtRequirement.
//
// Note: since the rbac filter is in charge of making decisions of allow/denied, this
// requirement uses `allow_missing_or_failed` to ensure it is always satisfied.
func providerToJWTRequirement(provider *structs.JWTProviderConfigEntry) *envoy_http_jwt_authn_v3.JwtRequirement {
	return &envoy_http_jwt_authn_v3.JwtRequirement{
		RequiresType: &envoy_http_jwt_authn_v3.JwtRequirement_RequiresAny{
			RequiresAny: &envoy_http_jwt_authn_v3.JwtRequirementOrList{
				Requirements: []*envoy_http_jwt_authn_v3.JwtRequirement{
					{
						RequiresType: &envoy_http_jwt_authn_v3.JwtRequirement_ProviderName{
							ProviderName: provider.Name,
						},
					},
					// We use allowMissingOrFailed to allow rbac filter to do the validation
					{
						RequiresType: &envoy_http_jwt_authn_v3.JwtRequirement_AllowMissingOrFailed{
							AllowMissingOrFailed: &emptypb.Empty{},
						},
					},
				},
			},
		},
	}
}

// collectJWTProviders returns a list of all top level and permission level referenced providers.
func collectJWTProviders(i *structs.Intention) []*structs.IntentionJWTProvider {
	// get permission level providers
	reqs := getPermissionsProviders(i.Permissions)

	if i.JWT != nil {
		// get top level providers
		reqs = append(reqs, i.JWT.Providers...)
	}

	return reqs
}

func getPermissionsProviders(perms []*structs.IntentionPermission) []*structs.IntentionJWTProvider {
	var reqs []*structs.IntentionJWTProvider
	for _, p := range perms {
		if p.JWT == nil {
			continue
		}

		reqs = append(reqs, p.JWT.Providers...)
	}

	return reqs
}

// buildPayloadInMetadataKey is used to create a unique payload key per provider.
// This is to ensure claims are validated/forwarded specifically under the right provider.
// The forwarded payload is used with other data (eg. service identity) by the RBAC filter
// to validate access to resource.
//
// eg. With a provider named okta will have a payload key of: jwt_payload_okta
func buildPayloadInMetadataKey(providerName string) string {
	return jwtMetadataKeyPrefix + "_" + providerName
}

func buildJWTProviderConfig(p *structs.JWTProviderConfigEntry) (*envoy_http_jwt_authn_v3.JwtProvider, error) {
	envoyCfg := envoy_http_jwt_authn_v3.JwtProvider{
		Issuer:            p.Issuer,
		Audiences:         p.Audiences,
		PayloadInMetadata: buildPayloadInMetadataKey(p.Name),
	}

	if p.Forwarding != nil {
		envoyCfg.ForwardPayloadHeader = p.Forwarding.HeaderName
		envoyCfg.PadForwardPayloadHeader = p.Forwarding.PadForwardPayloadHeader
	}

	if local := p.JSONWebKeySet.Local; local != nil {
		specifier, err := makeLocalJWKS(local, p.Name)
		if err != nil {
			return nil, err
		}
		envoyCfg.JwksSourceSpecifier = specifier
	} else if remote := p.JSONWebKeySet.Remote; remote != nil && remote.URI != "" {
		envoyCfg.JwksSourceSpecifier = makeRemoteJWKS(remote, p.Name)
	} else {
		return nil, fmt.Errorf("invalid jwt provider config; missing JSONWebKeySet for provider: %s", p.Name)
	}

	for _, location := range p.Locations {
		if location.Header != nil {
			//only setting forward here because it is only useful for headers not the other options
			envoyCfg.Forward = location.Header.Forward
			envoyCfg.FromHeaders = append(envoyCfg.FromHeaders, &envoy_http_jwt_authn_v3.JwtHeader{
				Name:        location.Header.Name,
				ValuePrefix: location.Header.ValuePrefix,
			})
		} else if location.QueryParam != nil {
			envoyCfg.FromParams = append(envoyCfg.FromParams, location.QueryParam.Name)
		} else if location.Cookie != nil {
			envoyCfg.FromCookies = append(envoyCfg.FromCookies, location.Cookie.Name)
		}
	}

	return &envoyCfg, nil
}

func makeLocalJWKS(l *structs.LocalJWKS, pName string) (*envoy_http_jwt_authn_v3.JwtProvider_LocalJwks, error) {
	var specifier *envoy_http_jwt_authn_v3.JwtProvider_LocalJwks
	if l.JWKS != "" {
		decodedJWKS, err := base64.StdEncoding.DecodeString(l.JWKS)
		if err != nil {
			return nil, err
		}
		specifier = &envoy_http_jwt_authn_v3.JwtProvider_LocalJwks{
			LocalJwks: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: string(decodedJWKS),
				},
			},
		}
	} else if l.Filename != "" {
		specifier = &envoy_http_jwt_authn_v3.JwtProvider_LocalJwks{
			LocalJwks: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_Filename{
					Filename: l.Filename,
				},
			},
		}
	} else {
		return nil, fmt.Errorf("invalid jwt provider config; missing JWKS/Filename for local provider: %s", pName)
	}

	return specifier, nil
}

func makeRemoteJWKS(r *structs.RemoteJWKS, providerName string) *envoy_http_jwt_authn_v3.JwtProvider_RemoteJwks {
	remote_specifier := envoy_http_jwt_authn_v3.JwtProvider_RemoteJwks{
		RemoteJwks: &envoy_http_jwt_authn_v3.RemoteJwks{
			HttpUri: &envoy_core_v3.HttpUri{
				Uri:              r.URI,
				HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{Cluster: makeJWKSClusterName(providerName)},
			},
			AsyncFetch: &envoy_http_jwt_authn_v3.JwksAsyncFetch{
				FastListener: r.FetchAsynchronously,
			},
		},
	}
	timeOutSecond := int64(r.RequestTimeoutMs) / 1000
	remote_specifier.RemoteJwks.HttpUri.Timeout = &durationpb.Duration{Seconds: timeOutSecond}
	cacheDuration := int64(r.CacheDuration)
	if cacheDuration > 0 {
		remote_specifier.RemoteJwks.CacheDuration = &durationpb.Duration{Seconds: cacheDuration}
	}

	p := buildJWTRetryPolicy(r.RetryPolicy)
	if p != nil {
		remote_specifier.RemoteJwks.RetryPolicy = p
	}

	return &remote_specifier
}

func makeJWKSClusterName(providerName string) string {
	return fmt.Sprintf("%s_%s", jwksClusterPrefix, providerName)
}

func buildJWTRetryPolicy(r *structs.JWKSRetryPolicy) *envoy_core_v3.RetryPolicy {
	var pol envoy_core_v3.RetryPolicy
	if r == nil {
		return nil
	}

	if r.RetryPolicyBackOff != nil {
		pol.RetryBackOff = &envoy_core_v3.BackoffStrategy{
			BaseInterval: structs.DurationToProto(r.RetryPolicyBackOff.BaseInterval),
			MaxInterval:  structs.DurationToProto(r.RetryPolicyBackOff.MaxInterval),
		}
	}

	pol.NumRetries = &wrapperspb.UInt32Value{
		Value: uint32(r.NumRetries),
	}

	return &pol
}

func hasJWTconfig(p []*structs.IntentionPermission) bool {
	for _, perm := range p {
		if perm.JWT != nil {
			return true
		}
	}
	return false
}
