// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const (
	testIssuer   = "https://issuer.example.test/"
	testAudience = "consul-test"
)

// workloadClaims are the private claims the tests put in the JWT: who the
// workload is and what it may do. They are what binding rules select on.
type workloadClaims struct {
	Role string `json:"role"`
	Team string `json:"team"`
}

// jwtIssuer signs tokens with a private key whose public half is configured in
// the auth method.
type jwtIssuer struct {
	t       *testing.T
	pubKey  string
	privKey string
}

func newJWTIssuer(t *testing.T) *jwtIssuer {
	t.Helper()
	pub, priv, err := libutils.GenerateKey()
	require.NoError(t, err)
	return &jwtIssuer{t: t, pubKey: pub, privKey: priv}
}

func (i *jwtIssuer) claims() jwt.Claims {
	return jwt.Claims{
		Subject:   "workload-1",
		Issuer:    testIssuer,
		Audience:  jwt.Audience{testAudience},
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
}

func (i *jwtIssuer) sign(claims jwt.Claims, private workloadClaims) string {
	i.t.Helper()
	token, err := libutils.SignJWT(i.privKey, claims, private)
	require.NoError(i.t, err)
	return token
}

// valid returns a JWT the auth method accepts for a workload with the given role and team.
func (i *jwtIssuer) valid(role, team string) string {
	return i.sign(i.claims(), workloadClaims{Role: role, Team: team})
}

func (i *jwtIssuer) methodConfig() map[string]interface{} {
	return map[string]interface{}{
		"JWTValidationPubKeys": []string{i.pubKey},
		"BoundIssuer":          testIssuer,
		"BoundAudiences":       []string{testAudience},
		"ClaimMappings":        map[string]string{"role": "role", "team": "team"},
	}
}

func createMethod(t *testing.T, c *api.Client, name string, config map[string]interface{}) *api.ACLAuthMethod {
	t.Helper()
	method, _, err := c.ACL().AuthMethodCreate(&api.ACLAuthMethod{Name: name, Type: "jwt", Config: config}, nil)
	require.NoError(t, err)
	return method
}

func login(c *api.Client, method, bearer string) (*api.ACLToken, error) {
	token, _, err := c.ACL().Login(&api.ACLLoginParams{AuthMethod: method, BearerToken: bearer}, nil)
	return token, err
}

// TestACL_AuthMethodAndBindingRuleManagement covers creating and validating JWT
// auth methods and their binding rules.
func TestACL_AuthMethodAndBindingRuleManagement(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	acl := e.admin.ACL()
	issuer := newJWTIssuer(t)

	t.Run("a jwt auth method is created, read, listed and updated", func(t *testing.T) {
		created, _, err := acl.AuthMethodCreate(&api.ACLAuthMethod{
			Name:        "managed-workloads",
			Type:        "jwt",
			DisplayName: "Managed workloads",
			Description: "workload identity",
			MaxTokenTTL: 30 * time.Minute,
			Config:      issuer.methodConfig(),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, "jwt", created.Type)

		got, _, err := acl.AuthMethodRead("managed-workloads", nil)
		require.NoError(t, err)
		require.Equal(t, "Managed workloads", got.DisplayName)
		require.Equal(t, 30*time.Minute, got.MaxTokenTTL)
		require.Equal(t, testIssuer, got.Config["BoundIssuer"])

		list, _, err := acl.AuthMethodList(nil)
		require.NoError(t, err)
		var names []string
		for _, m := range list {
			names = append(names, m.Name)
		}
		require.Contains(t, names, "managed-workloads")

		got.Description = "updated"
		updated, _, err := acl.AuthMethodUpdate(got, nil)
		require.NoError(t, err)
		require.Equal(t, "updated", updated.Description)
	})

	t.Run("invalid auth methods are rejected", func(t *testing.T) {
		pub := issuer.pubKey
		for name, tc := range map[string]struct {
			method  *api.ACLAuthMethod
			wantErr string
		}{
			"an unknown type":                 {&api.ACLAuthMethod{Name: "mystery", Type: "carrier-pigeon"}, "Type should be one of"},
			"no name":                         {&api.ACLAuthMethod{Type: "jwt", Config: issuer.methodConfig()}, "Must specify a Name"},
			"a jwt method with no key source": {&api.ACLAuthMethod{Name: "no-keys", Type: "jwt", Config: map[string]interface{}{"BoundIssuer": testIssuer}}, "exactly one of"},
			"a jwt method with two key sources": {&api.ACLAuthMethod{Name: "two-sources", Type: "jwt", Config: map[string]interface{}{
				"JWTValidationPubKeys": []string{pub}, "JWKSURL": "https://keys.example.test/jwks"}}, "exactly one of"},
			"a public key that is not a key":  {&api.ACLAuthMethod{Name: "bad-key", Type: "jwt", Config: map[string]interface{}{"JWTValidationPubKeys": []string{"nope"}}}, "error parsing public key"},
			"a token lifetime under a minute": {&api.ACLAuthMethod{Name: "short-ttl", Type: "jwt", MaxTokenTTL: time.Second, Config: issuer.methodConfig()}, "cannot be less than 1m0s"},
			"an unknown token locality":       {&api.ACLAuthMethod{Name: "bad-locality", Type: "jwt", TokenLocality: "galactic", Config: issuer.methodConfig()}, "TokenLocality should be one of"},
		} {
			_, _, err := acl.AuthMethodCreate(tc.method, nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), tc.wantErr, name)

			stored, _, err := acl.AuthMethodRead(tc.method.Name, nil)
			if tc.method.Name != "" {
				require.NoError(t, err, name)
				require.Nil(t, stored, "a rejected method must not be stored: %s", name)
			}
		}

		method, _, err := acl.AuthMethodRead("managed-workloads", nil)
		require.NoError(t, err)
		method.Type = "kubernetes"
		_, _, err = acl.AuthMethodUpdate(method, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "immutable")
	})

	t.Run("binding rules attach roles, services, nodes and policies to a login", func(t *testing.T) {
		_, _, err := acl.PolicyCreate(&api.ACLPolicy{Name: "rule-policy", Rules: `key_prefix "x/" { policy = "read" }`}, nil)
		require.NoError(t, err)
		_, _, err = acl.RoleCreate(&api.ACLRole{Name: "rule-role"}, nil)
		require.NoError(t, err)

		for _, rule := range []*api.ACLBindingRule{
			{AuthMethod: "managed-workloads", BindType: api.BindingRuleBindTypeService, BindName: "${value.team}", Selector: `value.role == "service"`},
			{AuthMethod: "managed-workloads", BindType: api.BindingRuleBindTypeRole, BindName: "rule-role"},
			{AuthMethod: "managed-workloads", BindType: api.BindingRuleBindTypePolicy, BindName: "rule-policy"},
			{AuthMethod: "managed-workloads", BindType: api.BindingRuleBindTypeNode, BindName: "${value.team}-node"},
		} {
			created, _, err := acl.BindingRuleCreate(rule, nil)
			require.NoError(t, err, "%s rule", rule.BindType)

			got, _, err := acl.BindingRuleRead(created.ID, nil)
			require.NoError(t, err)
			require.Equal(t, rule.BindName, got.BindName)
			require.Equal(t, rule.BindType, got.BindType)
		}

		rules, _, err := acl.BindingRuleList("managed-workloads", nil)
		require.NoError(t, err)
		require.Len(t, rules, 4)
	})

	t.Run("invalid binding rules are rejected", func(t *testing.T) {
		for name, tc := range map[string]struct {
			rule    *api.ACLBindingRule
			wantErr string
		}{
			"a method that does not exist":   {&api.ACLBindingRule{AuthMethod: "ghost", BindType: api.BindingRuleBindTypeService, BindName: "x"}, "auth method"},
			"an unknown bind type":           {&api.ACLBindingRule{AuthMethod: "managed-workloads", BindType: "wizard", BindName: "x"}, "BindType"},
			"no bind name":                   {&api.ACLBindingRule{AuthMethod: "managed-workloads", BindType: api.BindingRuleBindTypeService}, "BindName"},
			"a selector that does not parse": {&api.ACLBindingRule{AuthMethod: "managed-workloads", BindType: api.BindingRuleBindTypeService, BindName: "x", Selector: `value.role ==`}, "Selector"},
		} {
			_, _, err := acl.BindingRuleCreate(tc.rule, nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), tc.wantErr, name)
		}
	})

	t.Run("deleting a method deletes its binding rules", func(t *testing.T) {
		createMethod(t, e.admin, "short-lived", issuer.methodConfig())
		_, _, err := acl.BindingRuleCreate(&api.ACLBindingRule{AuthMethod: "short-lived", BindType: api.BindingRuleBindTypeService, BindName: "x"}, nil)
		require.NoError(t, err)

		_, err = acl.AuthMethodDelete("short-lived", nil)
		require.NoError(t, err)

		method, _, err := acl.AuthMethodRead("short-lived", nil)
		require.NoError(t, err)
		require.Nil(t, method)
		rules, _, err := acl.BindingRuleList("short-lived", nil)
		require.NoError(t, err)
		require.Empty(t, rules)
	})
}

// TestACL_JWTLogin verifies the login flow end to end: a workload exchanges a
// signed JWT for a Consul token whose permissions come from binding rules, and
// anything wrong with the JWT is refused.
func TestACL_JWTLogin(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	acl := e.admin.ACL()
	issuer := newJWTIssuer(t)

	_, _, err := acl.PolicyCreate(&api.ACLPolicy{Name: "config-reader", Rules: `key_prefix "config/" { policy = "read" }`}, nil)
	require.NoError(t, err)
	_, _, err = acl.RoleCreate(&api.ACLRole{Name: "config-readers", Policies: []*api.ACLRolePolicyLink{{Name: "config-reader"}}}, nil)
	require.NoError(t, err)
	_, _, err = acl.PolicyCreate(&api.ACLPolicy{Name: "admin-writer", Rules: `key_prefix "" { policy = "write" }`}, nil)
	require.NoError(t, err)
	_, _, err = acl.RoleCreate(&api.ACLRole{Name: "admins", Policies: []*api.ACLRolePolicyLink{{Name: "admin-writer"}}}, nil)
	require.NoError(t, err)

	method, _, err := acl.AuthMethodCreate(&api.ACLAuthMethod{
		Name: "workloads", Type: "jwt", MaxTokenTTL: 10 * time.Minute, Config: issuer.methodConfig(),
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, method)

	for _, rule := range []*api.ACLBindingRule{
		{AuthMethod: "workloads", BindType: api.BindingRuleBindTypeRole, BindName: "config-readers"},
		{AuthMethod: "workloads", BindType: api.BindingRuleBindTypeRole, BindName: "admins", Selector: `value.role == "admin"`},
		{AuthMethod: "workloads", BindType: api.BindingRuleBindTypeService, BindName: "${value.team}", Selector: `value.team != ""`},
	} {
		_, _, err := acl.BindingRuleCreate(rule, nil)
		require.NoError(t, err)
	}

	for _, key := range []string{"config/app", "other/app"} {
		_, err := e.adminClient.KV().Put(&api.KVPair{Key: key, Value: []byte("v")}, nil)
		require.NoError(t, err)
	}

	t.Run("a valid JWT is exchanged for a token with the bound permissions", func(t *testing.T) {
		token, err := login(e.adminClient, "workloads", issuer.valid("reader", "payments"))
		require.NoError(t, err)

		require.Equal(t, "workloads", token.AuthMethod)
		require.True(t, token.Local, "login tokens are datacenter-local by default")
		require.NotNil(t, token.ExpirationTime, "the method's MaxTokenTTL bounds the token")
		require.WithinDuration(t, time.Now().Add(10*time.Minute), *token.ExpirationTime, time.Minute)

		var roles []string
		for _, r := range token.Roles {
			roles = append(roles, r.Name)
		}
		require.Equal(t, []string{"config-readers"}, roles, "the admin binding does not match a reader")
		require.Len(t, token.ServiceIdentities, 1)
		require.Equal(t, "payments", token.ServiceIdentities[0].ServiceName, "the service name was built from the team claim")

		c := e.tokenClient(t, token.SecretID)
		pair, _, err := c.KV().Get("config/app", nil)
		require.NoError(t, err)
		require.NotNil(t, pair)
		_, _, err = c.KV().Get("other/app", nil)
		requireDenied(t, err, "outside the bound policy")
		_, err = c.KV().Put(&api.KVPair{Key: "config/app", Value: []byte("x")}, nil)
		requireDenied(t, err, "a reader cannot write")

		require.NoError(t, registerService(c, "payments"))
		requireDenied(t, registerService(c, "billing"), "the identity is limited to the team's service")
	})

	t.Run("a selector gives different workloads different permissions", func(t *testing.T) {
		token, err := login(e.adminClient, "workloads", issuer.valid("admin", "platform"))
		require.NoError(t, err)

		var roles []string
		for _, r := range token.Roles {
			roles = append(roles, r.Name)
		}
		require.ElementsMatch(t, []string{"config-readers", "admins"}, roles)

		_, err = e.tokenClient(t, token.SecretID).KV().Put(&api.KVPair{Key: "other/app", Value: []byte("changed")}, nil)
		require.NoError(t, err, "an admin can write anywhere")
	})

	t.Run("login tokens are tied to their method and can be listed by it", func(t *testing.T) {
		list, _, err := acl.TokenListFiltered(api.ACLTokenFilterOptions{AuthMethod: "workloads"}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, list)
		for _, tok := range list {
			require.Equal(t, "workloads", tok.AuthMethod)
		}
	})

	t.Run("logging out revokes the token", func(t *testing.T) {
		token, err := login(e.adminClient, "workloads", issuer.valid("reader", "payments"))
		require.NoError(t, err)
		c := e.tokenClient(t, token.SecretID)

		_, _, err = c.KV().Get("config/app", nil)
		require.NoError(t, err)

		_, err = c.ACL().Logout(nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			_, _, err := c.KV().Get("config/app", nil)
			require.Error(r, err)
			require.Contains(r, err.Error(), "ACL not found")
		})
	})

	t.Run("only a login token can log out", func(t *testing.T) {
		plain, _, err := acl.TokenCreate(&api.ACLToken{Description: "not from a login"}, nil)
		require.NoError(t, err)

		_, err = e.tokenClient(t, plain.SecretID).ACL().Logout(nil)
		require.Error(t, err, "a token that did not come from a login cannot be revoked this way")
	})

	t.Run("a JWT that fails validation is refused", func(t *testing.T) {
		otherKey := newJWTIssuer(t)

		expired := issuer.claims()
		expired.NotBefore = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
		expired.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Hour))
		notYetValid := issuer.claims()
		notYetValid.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
		wrongAudience := issuer.claims()
		wrongAudience.Audience = jwt.Audience{"another-service"}
		wrongIssuer := issuer.claims()
		wrongIssuer.Issuer = "https://evil.example.test/"

		for name, tc := range map[string]struct {
			bearer  string
			wantErr string
		}{
			"signed with another key": {otherKey.valid("admin", "platform"), "no known key successfully validated the token signature"},
			"expired":                 {issuer.sign(expired, workloadClaims{Role: "admin", Team: "platform"}), "token is expired"},
			"not valid yet":           {issuer.sign(notYetValid, workloadClaims{Role: "admin", Team: "platform"}), "token not valid yet"},
			"for another audience":    {issuer.sign(wrongAudience, workloadClaims{Role: "admin", Team: "platform"}), "aud claim does not match any bound audience"},
			"from another issuer":     {issuer.sign(wrongIssuer, workloadClaims{Role: "admin", Team: "platform"}), "invalid issuer claim"},
			"not a JWT at all":        {"definitely-not-a-jwt", "error parsing token"},
		} {
			token, err := login(e.adminClient, "workloads", tc.bearer)
			require.Error(t, err, name)
			require.Nil(t, token, name)
			require.Contains(t, err.Error(), tc.wantErr, name)
		}
	})

	t.Run("a login with no matching binding rule is refused", func(t *testing.T) {
		createMethod(t, e.admin, "strict", issuer.methodConfig())
		_, _, err := acl.BindingRuleCreate(&api.ACLBindingRule{
			AuthMethod: "strict", BindType: api.BindingRuleBindTypeRole, BindName: "admins", Selector: `value.role == "admin"`,
		}, nil)
		require.NoError(t, err)

		token, err := login(e.adminClient, "strict", issuer.valid("reader", "payments"))
		require.Error(t, err)
		require.Nil(t, token)
		require.Contains(t, err.Error(), "403")

		_, err = login(e.adminClient, "strict", issuer.valid("admin", "platform"))
		require.NoError(t, err)
	})

	t.Run("an unknown method is refused", func(t *testing.T) {
		_, err := login(e.adminClient, "no-such-method", issuer.valid("admin", "platform"))

		require.Error(t, err)
	})

	t.Run("deleting a method revokes the tokens it issued", func(t *testing.T) {
		createMethod(t, e.admin, "revocable", issuer.methodConfig())
		_, _, err := acl.BindingRuleCreate(&api.ACLBindingRule{AuthMethod: "revocable", BindType: api.BindingRuleBindTypeRole, BindName: "config-readers"}, nil)
		require.NoError(t, err)

		token, err := login(e.adminClient, "revocable", issuer.valid("reader", "payments"))
		require.NoError(t, err)
		c := e.tokenClient(t, token.SecretID)
		_, _, err = c.KV().Get("config/app", nil)
		require.NoError(t, err)

		_, err = acl.AuthMethodDelete("revocable", nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			_, _, err := c.KV().Get("config/app", nil)
			require.Error(r, err)
		})
	})
}

// TestACL_LoginCLI covers `consul acl auth-method`, `consul acl binding-rule`,
// `consul login` and `consul logout`, the way an operator and a workload would
// use them.
func TestACL_LoginCLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	issuer := newJWTIssuer(t)
	admin := func(args string) string { return "CONSUL_HTTP_TOKEN=" + e.cluster.TokenBootstrap + " consul " + args }

	_, _, err := e.admin.ACL().PolicyCreate(&api.ACLPolicy{Name: "cli-config-reader", Rules: `key_prefix "config/" { policy = "read" }`}, nil)
	require.NoError(t, err)
	_, _, err = e.admin.ACL().RoleCreate(&api.ACLRole{Name: "cli-config-readers", Policies: []*api.ACLRolePolicyLink{{Name: "cli-config-reader"}}}, nil)
	require.NoError(t, err)
	_, err = e.adminClient.KV().Put(&api.KVPair{Key: "config/app", Value: []byte("v")}, nil)
	require.NoError(t, err)

	t.Run("an operator sets up a method and a binding rule", func(t *testing.T) {
		// The method configuration is passed as JSON, so write it to a file first.
		configJSON := `{"JWTValidationPubKeys":["` + escapeNewlines(issuer.pubKey) + `"],"BoundIssuer":"` + testIssuer + `","BoundAudiences":["` + testAudience + `"],"ClaimMappings":{"role":"role","team":"team"}}`
		res := e.shell(t, "cat > /tmp/method.json <<'EOF'\n"+configJSON+"\nEOF\n"+
			admin("acl auth-method create -type jwt -name cli-workloads -config @/tmp/method.json"))
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "cli-workloads")

		res = e.shell(t, admin(`acl binding-rule create -method cli-workloads -bind-type role -bind-name cli-config-readers -selector 'value.role==reader'`))
		require.Equal(t, 0, res.ExitCode, res.Output)

		res = e.shell(t, admin("acl auth-method list"))
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "cli-workloads")

		res = e.shell(t, admin("acl binding-rule list -method cli-workloads"))
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "cli-config-readers")
	})

	t.Run("a workload logs in with a JWT file and writes the token to a sink", func(t *testing.T) {
		res := e.shell(t, "printf '%s' '"+issuer.valid("reader", "payments")+"' > /tmp/bearer.jwt && "+
			"consul login -method cli-workloads -bearer-token-file /tmp/bearer.jwt -token-sink-file /tmp/login.token && cat /tmp/login.token")
		require.Equal(t, 0, res.ExitCode, res.Output)

		sink := e.shell(t, "cat /tmp/login.token")
		secret := strings.TrimSpace(sink.Output)
		require.NotEmpty(t, secret)

		c := e.tokenClient(t, secret)
		pair, _, err := c.KV().Get("config/app", nil)
		require.NoError(t, err)
		require.NotNil(t, pair, "the login token carries the bound role")

		res = e.shell(t, "consul logout -token-file /tmp/login.token")
		require.Equal(t, 0, res.ExitCode, res.Output)

		eventually(t, func(r *retry.R) {
			_, _, err := c.KV().Get("config/app", nil)
			require.Error(r, err)
		})
	})

	t.Run("a login that does not match a binding rule fails", func(t *testing.T) {
		res := e.shell(t, "printf '%s' '"+issuer.valid("stranger", "payments")+"' > /tmp/stranger.jwt && "+
			"consul login -method cli-workloads -bearer-token-file /tmp/stranger.jwt -token-sink-file /tmp/stranger.token")

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "Permission denied")
	})

	t.Run("a bad JWT fails with a reason", func(t *testing.T) {
		res := e.shell(t, "printf '%s' 'not-a-jwt' > /tmp/bad.jwt && consul login -method cli-workloads -bearer-token-file /tmp/bad.jwt -token-sink-file /tmp/bad.token")

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "error parsing token")
	})
}

// escapeNewlines makes a PEM block safe to embed in a JSON string.
func escapeNewlines(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", `\n`)
}
