// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ssoauth

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestJWT_NewValidator(t *testing.T) {
	nullLogger := hclog.NewNullLogger()
	type AM = *structs.ACLAuthMethod

	makeAuthMethod := func(typ string, f func(method AM)) *structs.ACLAuthMethod {
		method := &structs.ACLAuthMethod{
			Name:        "test-" + typ,
			Description: typ + " test",
			Type:        typ,
			Config:      map[string]interface{}{},
		}
		if f != nil {
			f(method)
		}
		return method
	}

	oidcServer := oidcauthtest.Start(t)

	// Note that we won't test ALL of the available config variations here.
	// The go-sso library has exhaustive tests.
	for name, tc := range map[string]struct {
		method    *structs.ACLAuthMethod
		expectErr string
	}{
		"wrong type": {makeAuthMethod("invalid", nil), `type should be`},
		"extra config": {makeAuthMethod("jwt", func(method AM) {
			method.Config["extra"] = "config"
		}), "has invalid keys"},
		"wrong type of key in config blob": {makeAuthMethod("jwt", func(method AM) {
			method.Config["JWKSURL"] = []int{12345}
		}), `'JWKSURL' expected type 'string', got unconvertible type '[]int'`},

		"normal jwt - static keys": {makeAuthMethod("jwt", func(method AM) {
			method.Config["BoundIssuer"] = "https://legit.issuer.internal/"
			pubKey, _ := oidcServer.SigningKeys()
			method.Config["JWTValidationPubKeys"] = []string{pubKey}
		}), ""},
		"normal jwt - jwks": {makeAuthMethod("jwt", func(method AM) {
			method.Config["JWKSURL"] = oidcServer.Addr() + "/certs"
			method.Config["JWKSCACert"] = oidcServer.CACert()
		}), ""},
		"normal jwt - oidc discovery": {makeAuthMethod("jwt", func(method AM) {
			method.Config["OIDCDiscoveryURL"] = oidcServer.Addr()
			method.Config["OIDCDiscoveryCACert"] = oidcServer.CACert()
		}), ""},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			v, err := NewValidator(nullLogger, tc.method)
			if tc.expectErr != "" {
				testutil.RequireErrorContains(t, err, tc.expectErr)
				require.Nil(t, v)
			} else {
				require.NoError(t, err)
				require.NotNil(t, v)
				v.Stop()
			}
		})
	}
}

func TestJWT_ValidateLogin(t *testing.T) {
	type mConfig = map[string]interface{}

	setup := func(t *testing.T, f func(config mConfig)) *Validator {
		t.Helper()

		config := map[string]interface{}{
			"JWTSupportedAlgs": []string{"ES256"},
			"ClaimMappings": map[string]string{
				"first_name":   "name",
				"/org/primary": "primary_org",
			},
			"ListClaimMappings": map[string]string{
				"https://consul.test/groups": "groups",
			},
			"BoundAudiences": []string{"https://consul.test"},
		}
		if f != nil {
			f(config)
		}

		method := &structs.ACLAuthMethod{
			Name:   "test-method",
			Type:   "jwt",
			Config: config,
		}

		nullLogger := hclog.NewNullLogger()
		v, err := NewValidator(nullLogger, method)
		require.NoError(t, err)
		return v
	}

	oidcServer := oidcauthtest.Start(t)
	pubKey, privKey := oidcServer.SigningKeys()

	cases := map[string]struct {
		f         func(config mConfig)
		issuer    string
		expectErr string
	}{
		"success - jwt static keys": {func(config mConfig) {
			config["BoundIssuer"] = "https://legit.issuer.internal/"
			config["JWTValidationPubKeys"] = []string{pubKey}
		},
			"https://legit.issuer.internal/",
			""},
		"success - jwt jwks": {func(config mConfig) {
			config["JWKSURL"] = oidcServer.Addr() + "/certs"
			config["JWKSCACert"] = oidcServer.CACert()
		},
			"https://legit.issuer.internal/",
			""},
		"success - jwt oidc discovery": {func(config mConfig) {
			config["OIDCDiscoveryURL"] = oidcServer.Addr()
			config["OIDCDiscoveryCACert"] = oidcServer.CACert()
		},
			oidcServer.Addr(),
			""},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			v := setup(t, tc.f)

			cl := jwt.Claims{
				Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
				Audience:  jwt.Audience{"https://consul.test"},
				Issuer:    tc.issuer,
				NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
				Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
			}

			type orgs struct {
				Primary string `json:"primary"`
			}

			privateCl := struct {
				FirstName string   `json:"first_name"`
				Org       orgs     `json:"org"`
				Groups    []string `json:"https://consul.test/groups"`
			}{
				FirstName: "jeff2",
				Org:       orgs{"engineering"},
				Groups:    []string{"foo", "bar"},
			}

			jwtData, err := oidcauthtest.SignJWT(privKey, cl, privateCl)
			require.NoError(t, err)

			id, err := v.ValidateLogin(context.Background(), jwtData)
			if tc.expectErr != "" {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)

				authmethod.RequireIdentityMatch(t, id, map[string]string{
					"value.name":        "jeff2",
					"value.primary_org": "engineering",
				},
					"value.name == jeff2",
					"value.name != jeff",
					"value.primary_org == engineering",
					"foo in list.groups",
					"bar in list.groups",
					"salt not in list.groups",
				)
			}
		})
	}
}

func TestNewIdentity(t *testing.T) {
	// This is only based on claim mappings, so we'll just use the JWT type
	// since that's cheaper to setup.
	cases := map[string]struct {
		claimMappings     map[string]string
		listClaimMappings map[string]string
		expectVars        map[string]string
		expectFilters     []string
	}{
		"nil":   {nil, nil, kv(), nil},
		"empty": {kv(), kv(), kv(), nil},
		"one value mapping": {
			kv("foo1", "val1"),
			kv(),
			kv("value.val1", ""),
			[]string{`value.val1 == ""`},
		},
		"one list mapping": {kv(),
			kv("foo2", "val2"),
			kv(),
			nil,
		},
		"one of each": {
			kv("foo1", "val1"),
			kv("foo2", "val2"),
			kv("value.val1", ""),
			[]string{`value.val1 == ""`},
		},
		"two value mappings": {
			kv("foo1", "val1", "foo2", "val2"),
			kv(),
			kv("value.val1", "", "value.val2", ""),
			[]string{`value.val1 == ""`, `value.val2 == ""`},
		},
	}
	pubKey, _ := oidcauthtest.SigningKeys()

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			method := &structs.ACLAuthMethod{
				Name: "test-method",
				Type: "jwt",
				Config: map[string]interface{}{
					"BoundIssuer":          "https://legit.issuer.internal/",
					"JWTValidationPubKeys": []string{pubKey},
					"ClaimMappings":        tc.claimMappings,
					"ListClaimMappings":    tc.listClaimMappings,
				},
			}
			nullLogger := hclog.NewNullLogger()
			v, err := NewValidator(nullLogger, method)
			require.NoError(t, err)

			id := v.NewIdentity()
			authmethod.RequireIdentityMatch(t, id, tc.expectVars, tc.expectFilters...)
		})
	}
}

func kv(a ...string) map[string]string {
	if len(a)%2 != 0 {
		panic("kv() requires even numbers of arguments")
	}
	m := make(map[string]string)
	for i := 0; i < len(a); i += 2 {
		m[a[i]] = a[i+1]
	}
	return m
}
