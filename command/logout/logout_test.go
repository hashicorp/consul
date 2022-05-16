package logout

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/testrpc"
)

func TestLogout_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestLogoutCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			initial_management = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	t.Run("no token specified", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (ACL not found)")
	})

	t.Run("logout of deleted token", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=" + fakeID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (ACL not found)")
	})

	plainToken, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	t.Run("logout of ordinary token", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=" + plainToken.SecretID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (Permission denied: token wasn't created via login)")
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
	{
		_, _, err := client.ACL().BindingRuleCreate(&api.ACLBindingRule{
			AuthMethod: "test",
			BindType:   api.BindingRuleBindTypeService,
			BindName:   "${serviceaccount.name}",
		},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
	}

	var loginTokenSecret string
	{
		tok, _, err := client.ACL().Login(&api.ACLLoginParams{
			AuthMethod:  "test",
			BearerToken: "demo-token",
		}, nil)
		require.NoError(t, err)

		loginTokenSecret = tok.SecretID
	}

	t.Run("logout of login token", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=" + loginTokenSecret,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())
	})
}

func TestLogoutCommand_k8s(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			initial_management = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	t.Run("no token specified", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (ACL not found)")
	})

	t.Run("logout of deleted token", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=" + fakeID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (ACL not found)")
	})

	plainToken, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	t.Run("logout of ordinary token", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=" + plainToken.SecretID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "403 (Permission denied: token wasn't created via login)")
	})

	// go to the trouble of creating a login token
	// require.NoError(t, ioutil.WriteFile(bearerTokenFile, []byte(acl.TestKubernetesJWT_B), 0600))

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
		},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
	}

	var loginTokenSecret string
	{
		tok, _, err := client.ACL().Login(&api.ACLLoginParams{
			AuthMethod:  "k8s",
			BearerToken: acl.TestKubernetesJWT_B,
		}, nil)
		require.NoError(t, err)

		loginTokenSecret = tok.SecretID
	}

	t.Run("logout of login token", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=" + loginTokenSecret,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())
	})
}
