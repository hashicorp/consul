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
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	jwtEnvoyFilter       = "envoy.filters.http.jwt_authn"
	jwtMetadataKeyPrefix = "jwt_payload"
	jwksClusterPrefix    = "jwks_cluster"
)

// This is an intermediate JWTProvider form used to associate
// unique payload keys to providers
type jwtAuthnProvider struct {
	ComputedName string
	Provider     *structs.IntentionJWTProvider
}

func makeJWTAuthFilter(pCE map[string]*structs.JWTProviderConfigEntry, intentions structs.SimplifiedIntentions) (*envoy_http_v3.HttpFilter, error) {
	providers := map[string]*envoy_http_jwt_authn_v3.JwtProvider{}
	var rules []*envoy_http_jwt_authn_v3.RequirementRule

	for _, intention := range intentions {
		if intention.JWT == nil && !hasJWTconfig(intention.Permissions) {
			continue
		}
		for _, jwtReq := range collectJWTAuthnProviders(intention) {
			if _, ok := providers[jwtReq.ComputedName]; ok {
				continue
			}

			jwtProvider, ok := pCE[jwtReq.Provider.Name]

			if !ok {
				return nil, fmt.Errorf("provider specified in intention does not exist. Provider name: %s", jwtReq.Provider.Name)
			}
			// If intention permissions use HTTP-match criteria with
			// VerifyClaims, then generate a clone of the jwt provider with a
			// unique key for payload_in_metadata. The RBAC filter relies on
			// the key to check the correct claims for the matched request.
			envoyCfg, err := buildJWTProviderConfig(jwtProvider, jwtReq.ComputedName)
			if err != nil {
				return nil, err
			}
			providers[jwtReq.ComputedName] = envoyCfg
		}

		for k, perm := range intention.Permissions {
			if perm.JWT == nil {
				continue
			}
			for _, prov := range perm.JWT.Providers {
				rule := buildRouteRule(prov, perm, "/", k)
				rules = append(rules, rule)
			}
		}

		if intention.JWT != nil {
			for _, provider := range intention.JWT.Providers {
				// The top-level provider applies to all requests.
				rule := buildRouteRule(provider, nil, "/", 0)
				rules = append(rules, rule)
			}
		}
	}

	if len(intentions) == 0 && len(providers) == 0 {
		//do not add jwt_authn filter when intentions don't have JWT
		return nil, nil
	}

	cfg := &envoy_http_jwt_authn_v3.JwtAuthentication{
		Providers: providers,
		Rules:     rules,
	}
	return makeEnvoyHTTPFilter(jwtEnvoyFilter, cfg)
}

func collectJWTAuthnProviders(i *structs.Intention) []*jwtAuthnProvider {
	var reqs []*jwtAuthnProvider

	if i.JWT != nil {
		for _, prov := range i.JWT.Providers {
			reqs = append(reqs, &jwtAuthnProvider{Provider: prov, ComputedName: makeComputedProviderName(prov.Name, nil, 0)})
		}
	}

	reqs = append(reqs, getPermissionsProviders(i.Permissions)...)

	return reqs
}

func getPermissionsProviders(p []*structs.IntentionPermission) []*jwtAuthnProvider {
	var reqs []*jwtAuthnProvider
	for k, perm := range p {
		if perm.JWT == nil {
			continue
		}
		for _, prov := range perm.JWT.Providers {
			reqs = append(reqs, &jwtAuthnProvider{Provider: prov, ComputedName: makeComputedProviderName(prov.Name, perm, k)})
		}
	}

	return reqs
}

// makeComputedProviderName is used to create names for unique provider per permission
// This is to stop jwt claims cross validation across permissions/providers.
//
// eg. If Permission x is the 3rd permission and has a provider of original name okta
// this function will return okta_3 as the computed provider name
func makeComputedProviderName(name string, perm *structs.IntentionPermission, idx int) string {
	if perm == nil {
		return name
	}
	return fmt.Sprintf("%s_%d", name, idx)
}

// buildPayloadInMetadataKey is used to create a unique payload key per provider/permissions.
// This is to ensure claims are validated/forwarded specifically under the right permission/path
// and ensure we don't accidentally validate claims from different permissions/providers.
//
// eg. With a provider named okta, the second permission in permission list will have a provider of:
// okta_2 and a payload key of: jwt_payload_okta_2. Whereas an okta provider with no specific permission
// will have a payload key of: jwt_payload_okta
func buildPayloadInMetadataKey(providerName string, perm *structs.IntentionPermission, idx int) string {
	return fmt.Sprintf("%s_%s", jwtMetadataKeyPrefix, makeComputedProviderName(providerName, perm, idx))
}

func buildJWTProviderConfig(p *structs.JWTProviderConfigEntry, metadataKeySuffix string) (*envoy_http_jwt_authn_v3.JwtProvider, error) {
	envoyCfg := envoy_http_jwt_authn_v3.JwtProvider{
		Issuer:            p.Issuer,
		Audiences:         p.Audiences,
		PayloadInMetadata: buildPayloadInMetadataKey(metadataKeySuffix, nil, 0),
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

func buildRouteRule(provider *structs.IntentionJWTProvider, perm *structs.IntentionPermission, defaultPrefix string, permIdx int) *envoy_http_jwt_authn_v3.RequirementRule {
	rule := &envoy_http_jwt_authn_v3.RequirementRule{
		Match: &envoy_route_v3.RouteMatch{
			PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: defaultPrefix},
		},
		RequirementType: &envoy_http_jwt_authn_v3.RequirementRule_Requires{
			Requires: &envoy_http_jwt_authn_v3.JwtRequirement{
				RequiresType: &envoy_http_jwt_authn_v3.JwtRequirement_ProviderName{
					ProviderName: makeComputedProviderName(provider.Name, perm, permIdx),
				},
			},
		},
	}

	if perm != nil && perm.HTTP != nil {
		if perm.HTTP.PathPrefix != "" {
			rule.Match.PathSpecifier = &envoy_route_v3.RouteMatch_Prefix{
				Prefix: perm.HTTP.PathPrefix,
			}
		}

		if perm.HTTP.PathExact != "" {
			rule.Match.PathSpecifier = &envoy_route_v3.RouteMatch_Path{
				Path: perm.HTTP.PathExact,
			}
		}

		if perm.HTTP.PathRegex != "" {
			rule.Match.PathSpecifier = &envoy_route_v3.RouteMatch_SafeRegex{
				SafeRegex: makeEnvoyRegexMatch(perm.HTTP.PathRegex),
			}
		}
	}

	return rule
}

func hasJWTconfig(p []*structs.IntentionPermission) bool {
	for _, perm := range p {
		if perm.JWT != nil {
			return true
		}
	}
	return false
}
