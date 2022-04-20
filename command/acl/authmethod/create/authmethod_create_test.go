package authmethodcreate

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"

	// activate testing auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/testauth"
)

func TestAuthMethodCreateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestAuthMethodCreateCommand(t *testing.T) {
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
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))
	client := a.Client()

	t.Run("type required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-type' flag")
	})

	t.Run("name required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-name' flag")
	})

	t.Run("invalid type", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=invalid",
			"-name=my-method",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Invalid Auth Method: Type should be one of")
	})

	t.Run("create testing", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-description=desc",
			"-display-name=display",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:        name,
			Type:        "testing",
			DisplayName: "display",
			Description: "desc",
		}
		require.Equal(t, expect, got)
	})

	t.Run("create testing with max token ttl", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-description=desc",
			"-display-name=display",
			"-max-token-ttl=5m",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: "+ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:        name,
			Type:        "testing",
			DisplayName: "display",
			Description: "desc",
			MaxTokenTTL: 5 * time.Minute,
		}
		require.Equal(t, expect, got)
	})

	t.Run("create testing with token type global", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-description=desc",
			"-display-name=display",
			"-token-locality=global",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: "+ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:          name,
			Type:          "testing",
			DisplayName:   "display",
			Description:   "desc",
			TokenLocality: "global",
		}
		require.Equal(t, expect, got)
	})
}

func TestAuthMethodCreateCommand_JSON(t *testing.T) {
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
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))
	client := a.Client()

	t.Run("type required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-format=json",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-type' flag")
	})

	t.Run("create testing", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-description=desc",
			"-display-name=display",
			"-format=json",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		out := ui.OutputWriter.String()

		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, out, name)

		var jsonOutput json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(out), &jsonOutput))

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:        name,
			Type:        "testing",
			DisplayName: "display",
			Description: "desc",
		}
		require.Equal(t, expect, got)
	})

	t.Run("create testing with max token ttl", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-description=desc",
			"-display-name=display",
			"-max-token-ttl=5m",
			"-format=json",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		out := ui.OutputWriter.String()

		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, out, name)

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:        name,
			Type:        "testing",
			DisplayName: "display",
			Description: "desc",
			MaxTokenTTL: 5 * time.Minute,
		}
		require.Equal(t, expect, got)

		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(out), &raw))
		delete(raw, "CreateIndex")
		delete(raw, "ModifyIndex")
		delete(raw, "Namespace")
		delete(raw, "Partition")

		require.Equal(t, map[string]interface{}{
			"Name":        name,
			"Type":        "testing",
			"DisplayName": "display",
			"Description": "desc",
			"MaxTokenTTL": "5m0s",
			"Config":      nil,
		}, raw)
	})

	t.Run("create testing with token type global", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-description=desc",
			"-display-name=display",
			"-token-locality=global",
			"-format=json",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		out := ui.OutputWriter.String()

		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, out, name)

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:          name,
			Type:          "testing",
			DisplayName:   "display",
			Description:   "desc",
			TokenLocality: "global",
		}
		require.Equal(t, expect, got)

		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(out), &raw))
		delete(raw, "CreateIndex")
		delete(raw, "ModifyIndex")
		delete(raw, "Namespace")
		delete(raw, "Partition")

		require.Equal(t, map[string]interface{}{
			"Name":          name,
			"Type":          "testing",
			"DisplayName":   "display",
			"Description":   "desc",
			"TokenLocality": "global",
			"Config":        nil,
		}, raw)
	})
}

func TestAuthMethodCreateCommand_k8s(t *testing.T) {
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
			initial_management = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))
	client := a.Client()

	t.Run("k8s host required", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=kubernetes",
			"-name", name,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-kubernetes-host' flag")
	})

	t.Run("k8s ca cert required", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=kubernetes",
			"-name", name,
			"-kubernetes-host=https://foo.internal:8443",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-kubernetes-ca-cert' flag")
	})

	ca := connect.TestCA(t, nil)

	t.Run("k8s jwt required", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=kubernetes",
			"-name", name,
			"-kubernetes-host=https://foo.internal:8443",
			"-kubernetes-ca-cert", ca.RootCert,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-kubernetes-service-account-jwt' flag")
	})

	t.Run("create k8s", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=kubernetes",
			"-name", name,
			"-kubernetes-host", "https://foo.internal:8443",
			"-kubernetes-ca-cert", ca.RootCert,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_A,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name: name,
			Type: "kubernetes",
			Config: map[string]interface{}{
				"Host":              "https://foo.internal:8443",
				"CACert":            ca.RootCert,
				"ServiceAccountJWT": acl.TestKubernetesJWT_A,
			},
		}
		require.Equal(t, expect, got)
	})

	caFile := filepath.Join(testDir, "ca.crt")
	require.NoError(t, ioutil.WriteFile(caFile, []byte(ca.RootCert), 0600))

	t.Run("create k8s with cert file", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=kubernetes",
			"-name", name,
			"-kubernetes-host", "https://foo.internal:8443",
			"-kubernetes-ca-cert", "@" + caFile,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_A,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name: name,
			Type: "kubernetes",
			Config: map[string]interface{}{
				"Host":              "https://foo.internal:8443",
				"CACert":            ca.RootCert,
				"ServiceAccountJWT": acl.TestKubernetesJWT_A,
			},
		}
		require.Equal(t, expect, got)
	})
}

func TestAuthMethodCreateCommand_config(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testDir := testutil.TempDir(t, "auth-method")

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			initial_management = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))
	client := a.Client()

	checkMethod := func(t *testing.T, methodName string) {

		method, _, err := client.ACL().AuthMethodRead(
			methodName,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "foo", method.Config["SessionID"])
	}

	t.Run("config file", func(t *testing.T) {
		name := getTestName(t)
		configFile := filepath.Join(testDir, "config.json")
		jsonConfig := `{"SessionID":"foo"}`
		require.NoError(t, ioutil.WriteFile(configFile, []byte(jsonConfig), 0644))

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-config=@" + configFile,
		}
		ui := cli.NewMockUi()
		cmd := New(ui)
		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		checkMethod(t, name)
	})

	t.Run("config std-in", func(t *testing.T) {
		name := getTestName(t)
		stdinR, stdinW := io.Pipe()
		ui := cli.NewMockUi()
		cmd := New(ui)
		cmd.testStdin = stdinR
		go func() {
			stdinW.Write([]byte(`{"SessionID":"foo"}`))
			stdinW.Close()
		}()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-config=-",
		}
		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		checkMethod(t, name)

	})
	t.Run("config string", func(t *testing.T) {
		name := getTestName(t)
		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-type=testing",
			"-name", name,
			"-config=" + `{"SessionID":"foo"}`,
		}
		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		checkMethod(t, name)
	})
}

func getTestMethod(t *testing.T, client *api.Client, methodName string) *api.ACLAuthMethod {
	t.Helper()

	method, _, err := client.ACL().AuthMethodRead(
		methodName,
		&api.QueryOptions{Token: "root"},
	)
	require.NoError(t, err)
	require.NotNil(t, method)

	// zero these out since we don't really care
	method.CreateIndex = 0
	method.ModifyIndex = 0

	if method.Namespace == "default" {
		method.Namespace = ""
	}
	if method.Partition == "default" {
		method.Partition = ""
	}

	return method
}

func getTestName(t *testing.T) string {
	t.Helper()

	id, err := uuid.GenerateUUID()
	require.NoError(t, err)
	return "test-" + id

}
