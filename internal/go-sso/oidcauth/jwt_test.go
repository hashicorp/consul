package oidcauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"
)

func setupForJWT(t *testing.T, authType int, f func(c *Config)) (*Authenticator, string) {
	t.Helper()

	config := &Config{
		Type:             TypeJWT,
		JWTSupportedAlgs: []string{oidc.ES256},
		ClaimMappings: map[string]string{
			"first_name":       "name",
			"/org/primary":     "primary_org",
			"/nested/Size":     "size",
			"Age":              "age",
			"Admin":            "is_admin",
			"/nested/division": "division",
			"/nested/remote":   "is_remote",
		},
		ListClaimMappings: map[string]string{
			"https://go-sso/groups": "groups",
		},
	}

	var issuer string
	switch authType {
	case authOIDCDiscovery:
		srv := oidcauthtest.Start(t)
		config.OIDCDiscoveryURL = srv.Addr()
		config.OIDCDiscoveryCACert = srv.CACert()

		issuer = config.OIDCDiscoveryURL

		// TODO(sso): is this a bug in vault?
		// config.BoundIssuer = issuer
	case authStaticKeys:
		pubKey, _ := oidcauthtest.SigningKeys()
		config.BoundIssuer = "https://legit.issuer.internal/"
		config.JWTValidationPubKeys = []string{pubKey}
		issuer = config.BoundIssuer
	case authJWKS:
		srv := oidcauthtest.Start(t)
		config.JWKSURL = srv.Addr() + "/certs"
		config.JWKSCACert = srv.CACert()

		issuer = "https://legit.issuer.internal/"

		// TODO(sso): is this a bug in vault?
		// config.BoundIssuer = issuer
	default:
		require.Fail(t, "inappropriate authType: %d", authType)
	}

	if f != nil {
		f(config)
	}

	require.NoError(t, config.Validate())

	oa, err := New(config, hclog.NewNullLogger())
	require.NoError(t, err)
	t.Cleanup(oa.Stop)

	return oa, issuer
}

func TestJWT_OIDC_Functions_Fail(t *testing.T) {
	t.Run("static", func(t *testing.T) {
		testJWT_OIDC_Functions_Fail(t, authStaticKeys)
	})
	t.Run("JWKS", func(t *testing.T) {
		testJWT_OIDC_Functions_Fail(t, authJWKS)
	})
	t.Run("oidc discovery", func(t *testing.T) {
		testJWT_OIDC_Functions_Fail(t, authOIDCDiscovery)
	})
}

func testJWT_OIDC_Functions_Fail(t *testing.T, authType int) {
	t.Helper()

	t.Run("GetAuthCodeURL", func(t *testing.T) {
		oa, _ := setupForJWT(t, authType, nil)

		_, err := oa.GetAuthCodeURL(
			context.Background(),
			"https://example.com",
			map[string]string{"foo": "bar"},
		)
		requireErrorContains(t, err, `GetAuthCodeURL is incompatible with type "jwt"`)
	})

	t.Run("ClaimsFromAuthCode", func(t *testing.T) {
		oa, _ := setupForJWT(t, authType, nil)

		_, _, err := oa.ClaimsFromAuthCode(
			context.Background(),
			"abc", "def",
		)
		requireErrorContains(t, err, `ClaimsFromAuthCode is incompatible with type "jwt"`)
	})
}

func TestJWT_ClaimsFromJWT(t *testing.T) {
	t.Run("static", func(t *testing.T) {
		testJWT_ClaimsFromJWT(t, authStaticKeys)
	})
	t.Run("JWKS", func(t *testing.T) {
		testJWT_ClaimsFromJWT(t, authJWKS)
	})
	t.Run("oidc discovery", func(t *testing.T) {
		// TODO(sso): the vault versions of these tests did not run oidc-discovery
		testJWT_ClaimsFromJWT(t, authOIDCDiscovery)
	})
}

func testJWT_ClaimsFromJWT(t *testing.T, authType int) {
	t.Helper()

	t.Run("missing audience", func(t *testing.T) {
		if authType == authOIDCDiscovery {
			// TODO(sso): why isn't this strict?
			t.Skip("why?")
			return
		}
		oa, issuer := setupForJWT(t, authType, nil)

		cl := jwt.Claims{
			Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
			Issuer:    issuer,
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Audience:  jwt.Audience{"https://go-sso.test"},
			Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
		}

		privateCl := struct {
			User   string   `json:"https://go-sso/user"`
			Groups []string `json:"https://go-sso/groups"`
		}{
			"jeff",
			[]string{"foo", "bar"},
		}

		jwtData, err := oidcauthtest.SignJWT("", cl, privateCl)
		require.NoError(t, err)

		_, err = oa.ClaimsFromJWT(context.Background(), jwtData)
		requireErrorContains(t, err, "audience claim found in JWT but no audiences are bound")
	})

	t.Run("valid inputs", func(t *testing.T) {
		oa, issuer := setupForJWT(t, authType, func(c *Config) {
			c.BoundAudiences = []string{
				"https://go-sso.test",
				"another_audience",
			}
		})

		cl := jwt.Claims{
			Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
			Issuer:    issuer,
			Audience:  jwt.Audience{"https://go-sso.test"},
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
		}

		type orgs struct {
			Primary string `json:"primary"`
		}

		type nested struct {
			Division int64  `json:"division"`
			Remote   bool   `json:"remote"`
			Size     string `json:"Size"`
		}

		privateCl := struct {
			User      string   `json:"https://go-sso/user"`
			Groups    []string `json:"https://go-sso/groups"`
			FirstName string   `json:"first_name"`
			Org       orgs     `json:"org"`
			Color     string   `json:"color"`
			Age       int64    `json:"Age"`
			Admin     bool     `json:"Admin"`
			Nested    nested   `json:"nested"`
		}{
			User:      "jeff",
			Groups:    []string{"foo", "bar"},
			FirstName: "jeff2",
			Org:       orgs{"engineering"},
			Color:     "green",
			Age:       85,
			Admin:     true,
			Nested: nested{
				Division: 3,
				Remote:   true,
				Size:     "medium",
			},
		}

		jwtData, err := oidcauthtest.SignJWT("", cl, privateCl)
		require.NoError(t, err)

		claims, err := oa.ClaimsFromJWT(context.Background(), jwtData)
		require.NoError(t, err)

		expectedClaims := &Claims{
			Values: map[string]string{
				"name":        "jeff2",
				"primary_org": "engineering",
				"size":        "medium",
				"age":         "85",
				"is_admin":    "true",
				"division":    "3",
				"is_remote":   "true",
			},
			Lists: map[string][]string{
				"groups": {"foo", "bar"},
			},
		}

		require.Equal(t, expectedClaims, claims)
	})

	t.Run("unusable claims", func(t *testing.T) {
		oa, issuer := setupForJWT(t, authType, func(c *Config) {
			c.BoundAudiences = []string{
				"https://go-sso.test",
				"another_audience",
			}
		})

		cl := jwt.Claims{
			Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
			Issuer:    issuer,
			Audience:  jwt.Audience{"https://go-sso.test"},
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
		}

		type orgs struct {
			Primary string `json:"primary"`
		}

		type nested struct {
			Division int64    `json:"division"`
			Remote   bool     `json:"remote"`
			Size     []string `json:"Size"`
		}

		privateCl := struct {
			User      string   `json:"https://go-sso/user"`
			Groups    []string `json:"https://go-sso/groups"`
			FirstName string   `json:"first_name"`
			Org       orgs     `json:"org"`
			Color     string   `json:"color"`
			Age       int64    `json:"Age"`
			Admin     bool     `json:"Admin"`
			Nested    nested   `json:"nested"`
		}{
			User:      "jeff",
			Groups:    []string{"foo", "bar"},
			FirstName: "jeff2",
			Org:       orgs{"engineering"},
			Color:     "green",
			Age:       85,
			Admin:     true,
			Nested: nested{
				Division: 3,
				Remote:   true,
				Size:     []string{"medium"},
			},
		}

		jwtData, err := oidcauthtest.SignJWT("", cl, privateCl)
		require.NoError(t, err)

		_, err = oa.ClaimsFromJWT(context.Background(), jwtData)
		requireErrorContains(t, err, "error converting claim '/nested/Size' to string from unknown type []interface {}")
	})

	t.Run("bad signature", func(t *testing.T) {
		oa, issuer := setupForJWT(t, authType, func(c *Config) {
			c.BoundAudiences = []string{
				"https://go-sso.test",
				"another_audience",
			}
		})

		cl := jwt.Claims{
			Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
			Issuer:    issuer,
			Audience:  jwt.Audience{"https://go-sso.test"},
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
		}

		privateCl := struct {
			User   string   `json:"https://go-sso/user"`
			Groups []string `json:"https://go-sso/groups"`
		}{
			"jeff",
			[]string{"foo", "bar"},
		}

		jwtData, err := oidcauthtest.SignJWT(badPrivKey, cl, privateCl)
		require.NoError(t, err)

		_, err = oa.ClaimsFromJWT(context.Background(), jwtData)

		switch authType {
		case authOIDCDiscovery, authJWKS:
			requireErrorContains(t, err, "failed to verify id token signature")
		case authStaticKeys:
			requireErrorContains(t, err, "no known key successfully validated the token signature")
		default:
			require.Fail(t, "unexpected type: %d", authType)
		}
	})

	t.Run("bad issuer", func(t *testing.T) {
		oa, _ := setupForJWT(t, authType, func(c *Config) {
			c.BoundAudiences = []string{
				"https://go-sso.test",
				"another_audience",
			}
		})

		cl := jwt.Claims{
			Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
			Issuer:    "https://not.real.issuer.internal/",
			Audience:  jwt.Audience{"https://go-sso.test"},
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
		}

		privateCl := struct {
			User   string   `json:"https://go-sso/user"`
			Groups []string `json:"https://go-sso/groups"`
		}{
			"jeff",
			[]string{"foo", "bar"},
		}

		jwtData, err := oidcauthtest.SignJWT("", cl, privateCl)
		require.NoError(t, err)

		claims, err := oa.ClaimsFromJWT(context.Background(), jwtData)
		switch authType {
		case authOIDCDiscovery:
			requireErrorContains(t, err, "error validating signature: oidc: id token issued by a different provider")
		case authStaticKeys:
			requireErrorContains(t, err, "validation failed, invalid issuer claim (iss)")
		case authJWKS:
			// requireErrorContains(t, err, "validation failed, invalid issuer claim (iss)")
			// TODO(sso) The original vault test doesn't care about bound issuer.
			require.NoError(t, err)
			expectedClaims := &Claims{
				Values: map[string]string{},
				Lists: map[string][]string{
					"groups": {"foo", "bar"},
				},
			}
			require.Equal(t, expectedClaims, claims)
		default:
			require.Fail(t, "unexpected type: %d", authType)
		}
	})

	t.Run("bad audience", func(t *testing.T) {
		oa, issuer := setupForJWT(t, authType, func(c *Config) {
			c.BoundAudiences = []string{
				"https://go-sso.test",
				"another_audience",
			}
		})

		cl := jwt.Claims{
			Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
			Issuer:    issuer,
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Audience:  jwt.Audience{"https://fault.plugin.auth.jwt.test"},
			Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
		}

		privateCl := struct {
			User   string   `json:"https://go-sso/user"`
			Groups []string `json:"https://go-sso/groups"`
		}{
			"jeff",
			[]string{"foo", "bar"},
		}

		jwtData, err := oidcauthtest.SignJWT("", cl, privateCl)
		require.NoError(t, err)

		_, err = oa.ClaimsFromJWT(context.Background(), jwtData)
		requireErrorContains(t, err, "error validating claims: aud claim does not match any bound audience")
	})
}

func TestJWT_ClaimsFromJWT_ExpiryClaims(t *testing.T) {
	t.Run("static", func(t *testing.T) {
		t.Parallel()
		testJWT_ClaimsFromJWT_ExpiryClaims(t, authStaticKeys)
	})
	t.Run("JWKS", func(t *testing.T) {
		t.Parallel()
		testJWT_ClaimsFromJWT_ExpiryClaims(t, authJWKS)
	})
	// TODO(sso): the vault versions of these tests did not run oidc-discovery
	// t.Run("oidc discovery", func(t *testing.T) {
	// 	t.Parallel()
	// 	testJWT_ClaimsFromJWT_ExpiryClaims(t, authOIDCDiscovery)
	// })
}

func testJWT_ClaimsFromJWT_ExpiryClaims(t *testing.T, authType int) {
	t.Helper()

	tests := map[string]struct {
		Valid         bool
		IssuedAt      time.Time
		NotBefore     time.Time
		Expiration    time.Time
		DefaultLeeway int
		ExpLeeway     int
	}{
		// iat, auto clock_skew_leeway (60s), auto expiration leeway (150s)
		"auto expire leeway using iat with auto clock_skew_leeway":         {true, time.Now().Add(-205 * time.Second), time.Time{}, time.Time{}, 0, 0},
		"expired auto expire leeway using iat with auto clock_skew_leeway": {false, time.Now().Add(-215 * time.Second), time.Time{}, time.Time{}, 0, 0},

		// iat, clock_skew_leeway (10s), auto expiration leeway (150s)
		"auto expire leeway using iat with custom clock_skew_leeway":         {true, time.Now().Add(-150 * time.Second), time.Time{}, time.Time{}, 10, 0},
		"expired auto expire leeway using iat with custom clock_skew_leeway": {false, time.Now().Add(-165 * time.Second), time.Time{}, time.Time{}, 10, 0},

		// iat, no clock_skew_leeway (0s), auto expiration leeway (150s)
		"auto expire leeway using iat with no clock_skew_leeway":         {true, time.Now().Add(-145 * time.Second), time.Time{}, time.Time{}, -1, 0},
		"expired auto expire leeway using iat with no clock_skew_leeway": {false, time.Now().Add(-155 * time.Second), time.Time{}, time.Time{}, -1, 0},

		// nbf, auto clock_skew_leeway (60s), auto expiration leeway (150s)
		"auto expire leeway using nbf with auto clock_skew_leeway":         {true, time.Time{}, time.Now().Add(-205 * time.Second), time.Time{}, 0, 0},
		"expired auto expire leeway using nbf with auto clock_skew_leeway": {false, time.Time{}, time.Now().Add(-215 * time.Second), time.Time{}, 0, 0},

		// nbf, clock_skew_leeway (10s), auto expiration leeway (150s)
		"auto expire leeway using nbf with custom clock_skew_leeway":         {true, time.Time{}, time.Now().Add(-145 * time.Second), time.Time{}, 10, 0},
		"expired auto expire leeway using nbf with custom clock_skew_leeway": {false, time.Time{}, time.Now().Add(-165 * time.Second), time.Time{}, 10, 0},

		// nbf, no clock_skew_leeway (0s), auto expiration leeway (150s)
		"auto expire leeway using nbf with no clock_skew_leeway":         {true, time.Time{}, time.Now().Add(-145 * time.Second), time.Time{}, -1, 0},
		"expired auto expire leeway using nbf with no clock_skew_leeway": {false, time.Time{}, time.Now().Add(-155 * time.Second), time.Time{}, -1, 0},

		// iat, auto clock_skew_leeway (60s), custom expiration leeway (10s)
		"custom expire leeway using iat with clock_skew_leeway":         {true, time.Now().Add(-65 * time.Second), time.Time{}, time.Time{}, 0, 10},
		"expired custom expire leeway using iat with clock_skew_leeway": {false, time.Now().Add(-75 * time.Second), time.Time{}, time.Time{}, 0, 10},

		// iat, clock_skew_leeway (10s), custom expiration leeway (10s)
		"custom expire leeway using iat with clock_skew_leeway with default leeway":         {true, time.Now().Add(-5 * time.Second), time.Time{}, time.Time{}, 10, 10},
		"expired custom expire leeway using iat with clock_skew_leeway with default leeway": {false, time.Now().Add(-25 * time.Second), time.Time{}, time.Time{}, 10, 10},

		// iat, clock_skew_leeway (10s), no expiration leeway (10s)
		"no expire leeway using iat with clock_skew_leeway":         {true, time.Now().Add(-5 * time.Second), time.Time{}, time.Time{}, 10, -1},
		"expired no expire leeway using iat with clock_skew_leeway": {false, time.Now().Add(-15 * time.Second), time.Time{}, time.Time{}, 10, -1},

		// nbf, default clock_skew_leeway (60s), custom expiration leeway (10s)
		"custom expire leeway using nbf with clock_skew_leeway":         {true, time.Time{}, time.Now().Add(-65 * time.Second), time.Time{}, 0, 10},
		"expired custom expire leeway using nbf with clock_skew_leeway": {false, time.Time{}, time.Now().Add(-75 * time.Second), time.Time{}, 0, 10},

		// nbf, clock_skew_leeway (10s), custom expiration leeway (0s)
		"custom expire leeway using nbf with clock_skew_leeway with default leeway":         {true, time.Time{}, time.Now().Add(-5 * time.Second), time.Time{}, 10, 10},
		"expired custom expire leeway using nbf with clock_skew_leeway with default leeway": {false, time.Time{}, time.Now().Add(-25 * time.Second), time.Time{}, 10, 10},

		// nbf, clock_skew_leeway (10s), no expiration leeway (0s)
		"no expire leeway using nbf with clock_skew_leeway with default leeway":                 {true, time.Time{}, time.Now().Add(-5 * time.Second), time.Time{}, 10, -1},
		"no expire leeway using nbf with clock_skew_leeway with default leeway and nbf":         {true, time.Time{}, time.Now().Add(-5 * time.Second), time.Time{}, 10, -100},
		"expired no expire leeway using nbf with clock_skew_leeway":                             {false, time.Time{}, time.Now().Add(-15 * time.Second), time.Time{}, 10, -1},
		"expired no expire leeway using nbf with clock_skew_leeway with default leeway and nbf": {false, time.Time{}, time.Now().Add(-15 * time.Second), time.Time{}, 10, -100},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			oa, issuer := setupForJWT(t, authType, func(c *Config) {
				c.BoundAudiences = []string{
					"https://go-sso.test",
					"another_audience",
				}
				c.ClockSkewLeeway = time.Duration(tt.DefaultLeeway) * time.Second
				c.ExpirationLeeway = time.Duration(tt.ExpLeeway) * time.Second
				c.NotBeforeLeeway = 0
			})

			jwtData := setupLogin(t, tt.IssuedAt, tt.Expiration, tt.NotBefore, issuer)

			_, err := oa.ClaimsFromJWT(context.Background(), jwtData)
			if tt.Valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestJWT_ClaimsFromJWT_NotBeforeClaims(t *testing.T) {
	t.Run("static", func(t *testing.T) {
		t.Parallel()
		testJWT_ClaimsFromJWT_NotBeforeClaims(t, authStaticKeys)
	})
	t.Run("JWKS", func(t *testing.T) {
		t.Parallel()
		testJWT_ClaimsFromJWT_NotBeforeClaims(t, authJWKS)
	})
	// TODO(sso): the vault versions of these tests did not run oidc-discovery
	// t.Run("oidc discovery", func(t *testing.T) {
	// 	t.Parallel()
	// 	testJWT_ClaimsFromJWT_NotBeforeClaims(t, authOIDCDiscovery)
	// })
}

func testJWT_ClaimsFromJWT_NotBeforeClaims(t *testing.T, authType int) {
	t.Helper()

	tests := map[string]struct {
		Valid         bool
		IssuedAt      time.Time
		NotBefore     time.Time
		Expiration    time.Time
		DefaultLeeway int
		NBFLeeway     int
	}{
		// iat, auto clock_skew_leeway (60s), no nbf leeway (0)
		"no nbf leeway using iat with auto clock_skew_leeway":               {true, time.Now().Add(55 * time.Second), time.Time{}, time.Now(), 0, -1},
		"not yet valid no nbf leeway using iat with auto clock_skew_leeway": {false, time.Now().Add(65 * time.Second), time.Time{}, time.Now(), 0, -1},

		// iat, clock_skew_leeway (10s), no nbf leeway (0s)
		"no nbf leeway using iat with custom clock_skew_leeway":               {true, time.Now().Add(5 * time.Second), time.Time{}, time.Time{}, 10, -1},
		"not yet valid no nbf leeway using iat with custom clock_skew_leeway": {false, time.Now().Add(15 * time.Second), time.Time{}, time.Time{}, 10, -1},

		// iat, no clock_skew_leeway (0s), nbf leeway (5s)
		"nbf leeway using iat with no clock_skew_leeway":               {true, time.Now(), time.Time{}, time.Time{}, -1, 5},
		"not yet valid nbf leeway using iat with no clock_skew_leeway": {false, time.Now().Add(6 * time.Second), time.Time{}, time.Time{}, -1, 5},

		// exp, auto clock_skew_leeway (60s), auto nbf leeway (150s)
		"auto nbf leeway using exp with auto clock_skew_leeway":               {true, time.Time{}, time.Time{}, time.Now().Add(205 * time.Second), 0, 0},
		"not yet valid auto nbf leeway using exp with auto clock_skew_leeway": {false, time.Time{}, time.Time{}, time.Now().Add(215 * time.Second), 0, 0},

		// exp, clock_skew_leeway (10s), auto nbf leeway (150s)
		"auto nbf leeway using exp with custom clock_skew_leeway":               {true, time.Time{}, time.Time{}, time.Now().Add(150 * time.Second), 10, 0},
		"not yet valid auto nbf leeway using exp with custom clock_skew_leeway": {false, time.Time{}, time.Time{}, time.Now().Add(165 * time.Second), 10, 0},

		// exp, no clock_skew_leeway (0s), auto nbf leeway (150s)
		"auto nbf leeway using exp with no clock_skew_leeway":               {true, time.Time{}, time.Time{}, time.Now().Add(145 * time.Second), -1, 0},
		"not yet valid auto nbf leeway using exp with no clock_skew_leeway": {false, time.Time{}, time.Time{}, time.Now().Add(152 * time.Second), -1, 0},

		// exp, auto clock_skew_leeway (60s), custom nbf leeway (10s)
		"custom nbf leeway using exp with auto clock_skew_leeway":               {true, time.Time{}, time.Time{}, time.Now().Add(65 * time.Second), 0, 10},
		"not yet valid custom nbf leeway using exp with auto clock_skew_leeway": {false, time.Time{}, time.Time{}, time.Now().Add(75 * time.Second), 0, 10},

		// exp, clock_skew_leeway (10s), custom nbf leeway (10s)
		"custom nbf leeway using exp with custom clock_skew_leeway":               {true, time.Time{}, time.Time{}, time.Now().Add(15 * time.Second), 10, 10},
		"not yet valid custom nbf leeway using exp with custom clock_skew_leeway": {false, time.Time{}, time.Time{}, time.Now().Add(25 * time.Second), 10, 10},

		// exp, no clock_skew_leeway (0s), custom nbf leeway (5s)
		"custom nbf leeway using exp with no clock_skew_leeway":                                   {true, time.Time{}, time.Time{}, time.Now().Add(3 * time.Second), -1, 5},
		"custom nbf leeway using exp with no clock_skew_leeway with default leeway":               {true, time.Time{}, time.Time{}, time.Now().Add(3 * time.Second), -100, 5},
		"not yet valid custom nbf leeway using exp with no clock_skew_leeway":                     {false, time.Time{}, time.Time{}, time.Now().Add(7 * time.Second), -1, 5},
		"not yet valid custom nbf leeway using exp with no clock_skew_leeway with default leeway": {false, time.Time{}, time.Time{}, time.Now().Add(7 * time.Second), -100, 5},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			oa, issuer := setupForJWT(t, authType, func(c *Config) {
				c.BoundAudiences = []string{
					"https://go-sso.test",
					"another_audience",
				}
				c.ClockSkewLeeway = time.Duration(tt.DefaultLeeway) * time.Second
				c.ExpirationLeeway = 0
				c.NotBeforeLeeway = time.Duration(tt.NBFLeeway) * time.Second
			})

			jwtData := setupLogin(t, tt.IssuedAt, tt.Expiration, tt.NotBefore, issuer)

			_, err := oa.ClaimsFromJWT(context.Background(), jwtData)
			if tt.Valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func setupLogin(t *testing.T, iat, exp, nbf time.Time, issuer string) string {
	cl := jwt.Claims{
		Audience:  jwt.Audience{"https://go-sso.test"},
		Issuer:    issuer,
		Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
		IssuedAt:  jwt.NewNumericDate(iat),
		Expiry:    jwt.NewNumericDate(exp),
		NotBefore: jwt.NewNumericDate(nbf),
	}

	privateCl := struct {
		User   string   `json:"https://go-sso/user"`
		Groups []string `json:"https://go-sso/groups"`
		Color  string   `json:"color"`
	}{
		"foobar",
		[]string{"foo", "bar"},
		"green",
	}

	jwtData, err := oidcauthtest.SignJWT("", cl, privateCl)
	require.NoError(t, err)

	return jwtData
}

func TestParsePublicKeyPEM(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	getPublicPEM := func(t *testing.T, pub interface{}) string {
		derBytes, err := x509.MarshalPKIXPublicKey(pub)
		require.NoError(t, err)
		pemBlock := &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: derBytes,
		}
		return string(pem.EncodeToMemory(pemBlock))
	}

	t.Run("rsa", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		pub := privateKey.Public()
		pubPEM := getPublicPEM(t, pub)

		got, err := parsePublicKeyPEM([]byte(pubPEM))
		require.NoError(t, err)
		require.Equal(t, pub, got)
	})

	t.Run("ecdsa", func(t *testing.T) {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		pub := privateKey.Public()
		pubPEM := getPublicPEM(t, pub)

		got, err := parsePublicKeyPEM([]byte(pubPEM))
		require.NoError(t, err)
		require.Equal(t, pub, got)
	})

	t.Run("ed25519", func(t *testing.T) {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		pubPEM := getPublicPEM(t, pub)

		got, err := parsePublicKeyPEM([]byte(pubPEM))
		require.NoError(t, err)
		require.Equal(t, pub, got)
	})
}

const (
	badPrivKey string = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEILTAHJm+clBKYCrRDc74Pt7uF7kH+2x2TdL5cH23FEcsoAoGCCqGSM49
AwEHoUQDQgAE+C3CyjVWdeYtIqgluFJlwZmoonphsQbj9Nfo5wrEutv+3RTFnDQh
vttUajcFAcl4beR+jHFYC00vSO4i5jZ64g==
-----END EC PRIVATE KEY-----`
)
