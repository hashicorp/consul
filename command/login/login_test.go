package login

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestLoginCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestLoginCommand(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "acl")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, t.Name(), `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	a.Agent.LogWriter = logger.NewLogWriter(512)

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
		require.Contains(t, ui.ErrorWriter.String(), "403 (ACL not found)")
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
	t.Parallel()

	testDir := testutil.TempDir(t, "acl")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, t.Name(), `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	a.Agent.LogWriter = logger.NewLogWriter(512)

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
