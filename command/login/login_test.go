// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/hashicorp/consul-awsauth/iamauthtest"
	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestLoginCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestLoginCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testDir := testutil.TempDir(t, "acl")

	a := newTestAgent(t)
	client := a.Client()

	t.Run("method is required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-method' flag")
	})

	tokenSinkFile := filepath.Join(testDir, "test.token")

	t.Run("token-sink-file is required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-token-sink-file' flag")
	})

	t.Run("bearer-token-file is required", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-bearer-token-file' flag")
	})

	t.Run("bearer-token-file disallowed with aws-auto-bearer-token", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
			"-bearer-token-file", "none.txt",
			"-aws-auto-bearer-token",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Cannot use '-bearer-token-file' flag with '-aws-auto-bearer-token'")
	})

	t.Run("aws flags require aws-auto-bearer-token", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		baseArgs := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
		}

		for _, extraArgs := range [][]string{
			{"-aws-include-entity"},
			{"-aws-sts-endpoint", "some-endpoint"},
			{"-aws-region", "some-region"},
			{"-aws-server-id-header-value", "some-value"},
			{"-aws-access-key-id", "some-key"},
			{"-aws-secret-access-key", "some-secret"},
			{"-aws-session-token", "some-token"},
		} {
			ui := cli.NewMockUi()
			code := New(ui).Run(append(baseArgs, extraArgs...))
			require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
			require.Contains(t, ui.ErrorWriter.String(), "Missing '-aws-auto-bearer-token' flag")
		}
	})

	t.Run("aws-access-key-id and aws-secret-access-key require each other", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		baseArgs := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
			"-aws-auto-bearer-token",
		}

		ui := cli.NewMockUi()
		code := New(ui).Run(append(baseArgs, "-aws-access-key-id", "some-key"))
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing '-aws-secret-access-key' flag")

		ui = cli.NewMockUi()
		code = New(ui).Run(append(baseArgs, "-aws-secret-access-key", "some-key"))
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing '-aws-access-key-id' flag")

		ui = cli.NewMockUi()
		code = New(ui).Run(append(baseArgs, "-aws-session-token", "some-token"))
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(),
			"Missing '-aws-access-key-id' and '-aws-secret-access-key' flags")

	})

	bearerTokenFile := filepath.Join(testDir, "bearer.token")

	t.Run("bearer-token-file is empty", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		require.NoError(t, os.WriteFile(bearerTokenFile, []byte(""), 0600))

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
			"-bearer-token-file", bearerTokenFile,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "No bearer token found in")
	})

	require.NoError(t, os.WriteFile(bearerTokenFile, []byte("demo-token"), 0600))

	t.Run("try login with no method configured", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
			"-bearer-token-file", bearerTokenFile,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (ACL not found: auth method \"test\" not found")
	})

	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)

	testauth.InstallSessionToken(
		testSessionID,
		"demo-token",
		"default", "demo", "76091af4-4b56-11e9-ac4b-708b11801cbe",
	)

	{
		_, _, err := client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name: "test",
				Type: "testing",
				Config: map[string]interface{}{
					"SessionID": testSessionID,
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
	}

	t.Run("try login with method configured but no binding rules", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
			"-bearer-token-file", bearerTokenFile,
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (Permission denied)")
	})

	{
		_, _, err := client.ACL().BindingRuleCreate(&api.ACLBindingRule{
			AuthMethod: "test",
			BindType:   api.BindingRuleBindTypeService,
			BindName:   "${serviceaccount.name}",
			Selector:   "serviceaccount.namespace==default",
		},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
	}

	t.Run("try login with method configured and binding rules", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-token-sink-file", tokenSinkFile,
			"-bearer-token-file", bearerTokenFile,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())
		require.Empty(t, ui.OutputWriter.String())

		raw, err := os.ReadFile(tokenSinkFile)
		require.NoError(t, err)

		token := strings.TrimSpace(string(raw))
		require.Len(t, token, 36, "must be a valid uid: %s", token)
	})
}

func TestLoginCommand_k8s(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testDir := testutil.TempDir(t, "acl")

	a := newTestAgent(t)
	client := a.Client()

	tokenSinkFile := filepath.Join(testDir, "test.token")
	bearerTokenFile := filepath.Join(testDir, "bearer.token")

	// the "B" jwt will be the one being reviewed
	require.NoError(t, os.WriteFile(bearerTokenFile, []byte(acl.TestKubernetesJWT_B), 0600))

	// spin up a fake api server
	testSrv := kubeauth.StartTestAPIServer(t)
	defer testSrv.Stop()

	testSrv.AuthorizeJWT(acl.TestKubernetesJWT_A)
	testSrv.SetAllowedServiceAccount(
		"default",
		"demo",
		"76091af4-4b56-11e9-ac4b-708b11801cbe",
		"",
		acl.TestKubernetesJWT_B,
	)

	{
		_, _, err := client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name: "k8s",
				Type: "kubernetes",
				Config: map[string]interface{}{
					"Host":   testSrv.Addr(),
					"CACert": testSrv.CACert(),
					// the "A" jwt will be the one with token review privs
					"ServiceAccountJWT": acl.TestKubernetesJWT_A,
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
	}

	{
		_, _, err := client.ACL().BindingRuleCreate(&api.ACLBindingRule{
			AuthMethod: "k8s",
			BindType:   api.BindingRuleBindTypeService,
			BindName:   "${serviceaccount.name}",
			Selector:   "serviceaccount.namespace==default",
		},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
	}

	t.Run("try login with method configured and binding rules", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=k8s",
			"-token-sink-file", tokenSinkFile,
			"-bearer-token-file", bearerTokenFile,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())
		require.Empty(t, ui.OutputWriter.String())

		raw, err := os.ReadFile(tokenSinkFile)
		require.NoError(t, err)

		token := strings.TrimSpace(string(raw))
		require.Len(t, token, 36, "must be a valid uid: %s", token)
	})
}

func TestLoginCommand_jwt(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testDir := testutil.TempDir(t, "acl")

	a := newTestAgent(t)
	client := a.Client()

	tokenSinkFile := filepath.Join(testDir, "test.token")
	bearerTokenFile := filepath.Join(testDir, "bearer.token")

	// spin up a fake oidc server
	oidcServer := oidcauthtest.Start(t)
	pubKey, privKey := oidcServer.SigningKeys()

	type mConfig = map[string]interface{}
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
			method := &api.ACLAuthMethod{
				Name: "jwt",
				Type: "jwt",
				Config: map[string]interface{}{
					"JWTSupportedAlgs": []string{"ES256"},
					"ClaimMappings": map[string]string{
						"first_name":   "name",
						"/org/primary": "primary_org",
					},
					"ListClaimMappings": map[string]string{
						"https://consul.test/groups": "groups",
					},
					"BoundAudiences": []string{"https://consul.test"},
				},
			}
			if tc.f != nil {
				tc.f(method.Config)
			}
			_, _, err := client.ACL().AuthMethodCreate(
				method,
				&api.WriteOptions{Token: "root"},
			)
			require.NoError(t, err)

			_, _, err = client.ACL().BindingRuleCreate(&api.ACLBindingRule{
				AuthMethod: "jwt",
				BindType:   api.BindingRuleBindTypeService,
				BindName:   "test--${value.name}--${value.primary_org}",
				Selector:   "value.name == jeff2 and value.primary_org == engineering and foo in list.groups",
			},
				&api.WriteOptions{Token: "root"},
			)
			require.NoError(t, err)

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

			// Drop a JWT on disk.
			jwtData, err := oidcauthtest.SignJWT(privKey, cl, privateCl)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(bearerTokenFile, []byte(jwtData), 0600))

			defer os.Remove(tokenSinkFile)
			ui := cli.NewMockUi()
			cmd := New(ui)

			args := []string{
				"-http-addr=" + a.HTTPAddr(),
				"-token=root",
				"-method=jwt",
				"-token-sink-file", tokenSinkFile,
				"-bearer-token-file", bearerTokenFile,
			}

			code := cmd.Run(args)
			require.Equal(t, 0, code, "err: %s", ui.ErrorWriter.String())
			require.Empty(t, ui.ErrorWriter.String())
			require.Empty(t, ui.OutputWriter.String())

			raw, err := os.ReadFile(tokenSinkFile)
			require.NoError(t, err)

			token := strings.TrimSpace(string(raw))
			require.Len(t, token, 36, "must be a valid uid: %s", token)
		})
	}
}

func TestLoginCommand_aws_iam(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Formats an HIL template for a BindName, and the expected value for entity tags.
	// Input:   string{"a", "b"}, []string{"1", "2"}
	// Return: "${entity_tags.a}-${entity_tags.b}",  "1-2"
	entityTagsBind := func(keys, values []string) (string, string) {
		parts := []string{}
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("${entity_tags.%s}", k))
		}
		return strings.Join(parts, "-"), strings.Join(values, "-")
	}

	f := iamauthtest.MakeFixture()
	roleTagsBindName, roleTagsBindValue := entityTagsBind(f.RoleTagKeys(), f.RoleTagValues())
	userTagsBindName, userTagsBindValue := entityTagsBind(f.UserTagKeys(), f.UserTagValues())

	cases := map[string]struct {
		awsServer          *iamauthtest.Server
		cmdArgs            []string
		config             map[string]interface{}
		bindingRule        *api.ACLBindingRule
		expServiceIdentity *api.ACLServiceIdentity
	}{
		"success - login with role": {
			awsServer: f.ServerForRole,
			cmdArgs:   []string{"-aws-auto-bearer-token"},
			config: map[string]interface{}{
				// Test that an assumed-role arn is translated to the canonical role arn.
				"BoundIAMPrincipalARNs": []string{f.CanonicalRoleARN},
			},
			bindingRule: &api.ACLBindingRule{
				BindType: api.BindingRuleBindTypeService,
				BindName: "${entity_name}-${entity_id}-${account_id}",
				Selector: fmt.Sprintf(`entity_name==%q and entity_id==%q and account_id==%q`,
					f.RoleName, f.EntityID, f.AccountID),
			},
			expServiceIdentity: &api.ACLServiceIdentity{
				ServiceName: fmt.Sprintf("%s-%s-%s", f.RoleName, strings.ToLower(f.EntityID), f.AccountID),
			},
		},
		"success - login with role and entity details enabled": {
			awsServer: f.ServerForRole,
			cmdArgs:   []string{"-aws-auto-bearer-token", "-aws-include-entity"},
			config: map[string]interface{}{
				// Test that we can login with full user path.
				"BoundIAMPrincipalARNs":  []string{f.RoleARN},
				"EnableIAMEntityDetails": true,
			},
			bindingRule: &api.ACLBindingRule{
				BindType: api.BindingRuleBindTypeService,
				// TODO: Path cannot be used as service name if it contains a '/'
				BindName: "${entity_name}",
				Selector: fmt.Sprintf(`entity_name==%q and entity_path==%q`, f.RoleName, f.RolePath),
			},
			expServiceIdentity: &api.ACLServiceIdentity{ServiceName: f.RoleName},
		},
		"success - login with role and role tags": {
			awsServer: f.ServerForRole,
			cmdArgs:   []string{"-aws-auto-bearer-token", "-aws-include-entity"},
			config: map[string]interface{}{
				// Test that we can login with a wildcard.
				"BoundIAMPrincipalARNs":  []string{f.RoleARNWildcard},
				"EnableIAMEntityDetails": true,
				"IAMEntityTags":          f.RoleTagKeys(),
			},
			bindingRule: &api.ACLBindingRule{
				BindType: api.BindingRuleBindTypeService,
				BindName: roleTagsBindName,
				Selector: fmt.Sprintf(`entity_name==%q and entity_path==%q`, f.RoleName, f.RolePath),
			},
			expServiceIdentity: &api.ACLServiceIdentity{ServiceName: roleTagsBindValue},
		},
		"success - login with user and user tags": {
			awsServer: f.ServerForUser,
			cmdArgs:   []string{"-aws-auto-bearer-token", "-aws-include-entity"},
			config: map[string]interface{}{
				// Test that we can login with a wildcard.
				"BoundIAMPrincipalARNs":  []string{f.UserARNWildcard},
				"EnableIAMEntityDetails": true,
				"IAMEntityTags":          f.UserTagKeys(),
			},
			bindingRule: &api.ACLBindingRule{
				BindType: api.BindingRuleBindTypeService,
				BindName: "${entity_name}-" + userTagsBindName,
				Selector: fmt.Sprintf(`entity_name==%q and entity_path==%q`, f.UserName, f.UserPath),
			},
			expServiceIdentity: &api.ACLServiceIdentity{
				ServiceName: fmt.Sprintf("%s-%s", f.UserName, userTagsBindValue),
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := newTestAgent(t)
			client := a.Client()

			fakeAws := iamauthtest.NewTestServer(t, c.awsServer)

			c.config["STSEndpoint"] = fakeAws.URL + "/sts"
			c.config["IAMEndpoint"] = fakeAws.URL + "/iam"

			_, _, err := client.ACL().AuthMethodCreate(
				&api.ACLAuthMethod{
					Name:   "iam-test",
					Type:   "aws-iam",
					Config: c.config,
				},
				&api.WriteOptions{Token: "root"},
			)
			require.NoError(t, err)

			c.bindingRule.AuthMethod = "iam-test"
			_, _, err = client.ACL().BindingRuleCreate(
				c.bindingRule,
				&api.WriteOptions{Token: "root"},
			)
			require.NoError(t, err)

			testDir := testutil.TempDir(t, "acl")
			tokenSinkFile := filepath.Join(testDir, "test.token")
			t.Cleanup(func() { _ = os.Remove(tokenSinkFile) })

			ui := cli.NewMockUi()
			cmd := New(ui)
			args := []string{
				"-http-addr=" + a.HTTPAddr(),
				"-token=root",
				"-method=iam-test",
				"-token-sink-file", tokenSinkFile,
				"-aws-sts-endpoint", fakeAws.URL + "/sts",
				"-aws-region", "fake-region",
				"-aws-access-key-id", "fake-key-id",
				"-aws-secret-access-key", "fake-secret-key",
			}
			args = append(args, c.cmdArgs...)
			code := cmd.Run(args)
			require.Equal(t, 0, code, ui.ErrorWriter.String())

			raw, err := os.ReadFile(tokenSinkFile)
			require.NoError(t, err)

			token := strings.TrimSpace(string(raw))
			require.Len(t, token, 36, "must be a valid uid: %s", token)

			// Validate correct BindName was interpolated.
			tokenRead, _, err := client.ACL().TokenReadSelf(&api.QueryOptions{Token: token})
			require.NoError(t, err)
			require.Len(t, tokenRead.ServiceIdentities, 1)
			require.Equal(t, c.expServiceIdentity, tokenRead.ServiceIdentities[0])

		})
	}
}

func newTestAgent(t *testing.T) *agent.TestAgent {
	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			initial_management = "root"
		}
	}`)
	t.Cleanup(func() { _ = a.Shutdown() })
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	return a
}
