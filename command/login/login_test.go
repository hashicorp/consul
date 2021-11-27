package login

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"
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

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

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

	bearerTokenFile := filepath.Join(testDir, "bearer.token")

	t.Run("bearer-token-file is empty", func(t *testing.T) {
		defer os.Remove(tokenSinkFile)

		require.NoError(t, ioutil.WriteFile(bearerTokenFile, []byte(""), 0600))

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

	require.NoError(t, ioutil.WriteFile(bearerTokenFile, []byte("demo-token"), 0600))

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

		raw, err := ioutil.ReadFile(tokenSinkFile)
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

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	tokenSinkFile := filepath.Join(testDir, "test.token")
	bearerTokenFile := filepath.Join(testDir, "bearer.token")

	// the "B" jwt will be the one being reviewed
	require.NoError(t, ioutil.WriteFile(bearerTokenFile, []byte(acl.TestKubernetesJWT_B), 0600))

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

		raw, err := ioutil.ReadFile(tokenSinkFile)
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

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	tokenSinkFile := filepath.Join(testDir, "test.token")
	bearerTokenFile := filepath.Join(testDir, "bearer.token")

	// spin up a fake oidc server
	oidcServer := oidcauthtest.Start(t, oidcauthtest.WithPort(freeport.Port(t)))
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
			require.NoError(t, ioutil.WriteFile(bearerTokenFile, []byte(jwtData), 0600))

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

			raw, err := ioutil.ReadFile(tokenSinkFile)
			require.NoError(t, err)

			token := strings.TrimSpace(string(raw))
			require.Len(t, token, 36, "must be a valid uid: %s", token)
		})
	}
}
