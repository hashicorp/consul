// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authmethodupdate

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestAuthMethodUpdateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestAuthMethodUpdateCommand(t *testing.T) {
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

	t.Run("update without name", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Cannot update an auth method without specifying the -name parameter")
	})

	t.Run("update nonexistent method", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name", name,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Auth method not found with name")
	})

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "test-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "testing",
				Description: "test",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	finalName := createAuthMethod(t)

	t.Run("update all fields", func(t *testing.T) {
		name := finalName
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-display-name", "updated display",
			"-description", "updated description",
			"-config", `{ "SessionID": "foo" }`,
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
			DisplayName: "updated display",
			Description: "updated description",
			Config: map[string]interface{}{
				"SessionID": "foo",
			},
		}
		require.Equal(t, expect, got)
	})

	t.Run("update config field and prove no merging happens", func(t *testing.T) {
		name := finalName
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-display-name", "updated display",
			"-description", "updated description",
			"-config", `{ "Data": { "foo": "bar"} }`,
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
			DisplayName: "updated display",
			Description: "updated description",
			Config: map[string]interface{}{
				"Data": map[string]interface{}{
					"foo": "bar",
				},
			},
		}
		require.Equal(t, expect, got)
	})
}

func TestAuthMethodUpdateCommand_JSON(t *testing.T) {
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

	t.Run("update without name", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-format=json",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Cannot update an auth method without specifying the -name parameter")
	})

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "test-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "testing",
				Description: "test",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	t.Run("update all fields", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-display-name", "updated display",
			"-description", "updated description",
			"-format=json",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		output := ui.OutputWriter.String()

		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		var jsonOutput json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(output), &jsonOutput))

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:        name,
			Type:        "testing",
			DisplayName: "updated display",
			Description: "updated description",
			Config:      map[string]interface{}{},
		}
		require.Equal(t, expect, got)
	})
}

func TestAuthMethodUpdateCommand_noMerge(t *testing.T) {
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

	t.Run("update without name", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Cannot update an auth method without specifying the -name parameter")
	})

	t.Run("update nonexistent method", func(t *testing.T) {
		name := getTestName(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-name", name,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Auth method not found with name")
	})

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "test-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "testing",
				Description: "test",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	t.Run("update all fields", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-name=" + name,
			"-display-name", "updated display",
			"-description", "updated description",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:        name,
			Type:        "testing",
			DisplayName: "updated display",
			Description: "updated description",
		}
		require.Equal(t, expect, got)
	})
}

func TestAuthMethodUpdateCommand_k8s(t *testing.T) {
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
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	ca := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "k8s-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "kubernetes",
				Description: "test",
				Config: map[string]interface{}{
					"Host":              "https://foo.internal:8443",
					"CACert":            ca.RootCert,
					"ServiceAccountJWT": acl.TestKubernetesJWT_A,
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	t.Run("update all fields", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-display-name", "updated display",
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-ca-cert", ca2.RootCert,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		got := getTestMethod(t, client, name)
		expect := &api.ACLAuthMethod{
			Name:        name,
			Type:        "kubernetes",
			DisplayName: "updated display",
			Description: "updated description",
			Config: map[string]interface{}{
				"Host":              "https://foo-new.internal:8443",
				"CACert":            ca2.RootCert,
				"ServiceAccountJWT": acl.TestKubernetesJWT_B,
			},
		}
		require.Equal(t, expect, got)

		// also just double check our convenience parsing
		config, err := api.ParseKubernetesAuthMethodConfig(got.Config)
		require.NoError(t, err)
		require.Equal(t, "https://foo-new.internal:8443", config.Host)
		require.Equal(t, ca2.RootCert, config.CACert)
		require.Equal(t, acl.TestKubernetesJWT_B, config.ServiceAccountJWT)
	})

	ca2File := filepath.Join(testDir, "ca2.crt")
	require.NoError(t, os.WriteFile(ca2File, []byte(ca2.RootCert), 0600))

	t.Run("update all fields with cert file", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-ca-cert", "@" + ca2File,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		method, _, err := client.ACL().AuthMethodRead(
			name,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "updated description", method.Description)

		config, err := api.ParseKubernetesAuthMethodConfig(method.Config)
		require.NoError(t, err)

		require.Equal(t, "https://foo-new.internal:8443", config.Host)
		require.Equal(t, ca2.RootCert, config.CACert)
		require.Equal(t, acl.TestKubernetesJWT_B, config.ServiceAccountJWT)
	})

	t.Run("update all fields but k8s host", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-ca-cert", ca2.RootCert,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		method, _, err := client.ACL().AuthMethodRead(
			name,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "updated description", method.Description)

		config, err := api.ParseKubernetesAuthMethodConfig(method.Config)
		require.NoError(t, err)

		require.Equal(t, "https://foo.internal:8443", config.Host)
		require.Equal(t, ca2.RootCert, config.CACert)
		require.Equal(t, acl.TestKubernetesJWT_B, config.ServiceAccountJWT)
	})

	t.Run("update all fields but k8s ca cert", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		method, _, err := client.ACL().AuthMethodRead(
			name,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "updated description", method.Description)

		config, err := api.ParseKubernetesAuthMethodConfig(method.Config)
		require.NoError(t, err)

		require.Equal(t, "https://foo-new.internal:8443", config.Host)
		require.Equal(t, ca.RootCert, config.CACert)
		require.Equal(t, acl.TestKubernetesJWT_B, config.ServiceAccountJWT)
	})

	t.Run("update all fields but k8s jwt", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-ca-cert", ca2.RootCert,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		method, _, err := client.ACL().AuthMethodRead(
			name,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "updated description", method.Description)

		config, err := api.ParseKubernetesAuthMethodConfig(method.Config)
		require.NoError(t, err)

		require.Equal(t, "https://foo-new.internal:8443", config.Host)
		require.Equal(t, ca2.RootCert, config.CACert)
		require.Equal(t, acl.TestKubernetesJWT_A, config.ServiceAccountJWT)
	})
}

func TestAuthMethodUpdateCommand_k8s_noMerge(t *testing.T) {
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
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	ca := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "k8s-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "kubernetes",
				Description: "test",
				Config: map[string]interface{}{
					"Host":              "https://foo.internal:8443",
					"CACert":            ca.RootCert,
					"ServiceAccountJWT": acl.TestKubernetesJWT_A,
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	t.Run("update missing k8s host", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-ca-cert", ca2.RootCert,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-kubernetes-host' flag")
	})

	t.Run("update missing k8s ca cert", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-kubernetes-ca-cert' flag")
	})

	t.Run("update missing k8s jwt", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-ca-cert", ca2.RootCert,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-kubernetes-service-account-jwt' flag")
	})

	t.Run("update all fields", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-ca-cert", ca2.RootCert,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		method, _, err := client.ACL().AuthMethodRead(
			name,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "updated description", method.Description)

		config, err := api.ParseKubernetesAuthMethodConfig(method.Config)
		require.NoError(t, err)

		require.Equal(t, "https://foo-new.internal:8443", config.Host)
		require.Equal(t, ca2.RootCert, config.CACert)
		require.Equal(t, acl.TestKubernetesJWT_B, config.ServiceAccountJWT)
	})

	ca2File := filepath.Join(testDir, "ca2.crt")
	require.NoError(t, os.WriteFile(ca2File, []byte(ca2.RootCert), 0600))

	t.Run("update all fields with cert file", func(t *testing.T) {
		name := createAuthMethod(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-name=" + name,
			"-description", "updated description",
			"-kubernetes-host", "https://foo-new.internal:8443",
			"-kubernetes-ca-cert", "@" + ca2File,
			"-kubernetes-service-account-jwt", acl.TestKubernetesJWT_B,
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		method, _, err := client.ACL().AuthMethodRead(
			name,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "updated description", method.Description)

		config, err := api.ParseKubernetesAuthMethodConfig(method.Config)
		require.NoError(t, err)

		require.Equal(t, "https://foo-new.internal:8443", config.Host)
		require.Equal(t, ca2.RootCert, config.CACert)
		require.Equal(t, acl.TestKubernetesJWT_B, config.ServiceAccountJWT)
	})
}

func TestAuthMethodUpdateCommand_config(t *testing.T) {
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
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "test" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "testing",
				Description: "test",
				Config: map[string]interface{}{
					"SessionID": "big",
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	readUpdate := func(t *testing.T, methodName string) {

		method, _, err := client.ACL().AuthMethodRead(
			methodName,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, method)
		require.Equal(t, "update", method.Config["SessionID"])
	}

	t.Run("config file", func(t *testing.T) {
		methodName := createAuthMethod(t)
		configFile := filepath.Join(testDir, "config.json")
		jsonConfig := `{"SessionID":"update"}`
		require.NoError(t, os.WriteFile(configFile, []byte(jsonConfig), 0644))

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + methodName,
			"-no-merge=true",
			"-config=@" + configFile,
		}
		ui := cli.NewMockUi()
		cmd := New(ui)
		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		readUpdate(t, methodName)
	})

	t.Run("config stdin", func(t *testing.T) {
		methodName := createAuthMethod(t)
		ui := cli.NewMockUi()
		cmd := New(ui)
		stdinR, stdinW := io.Pipe()
		cmd.testStdin = stdinR

		go func() {
			stdinW.Write([]byte(`{"SessionID":"update"}`))
			stdinW.Close()
		}()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + methodName,
			"-no-merge=true",
			"-config=-",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		readUpdate(t, methodName)
	})

	t.Run("config string", func(t *testing.T) {
		methodName := createAuthMethod(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + methodName,
			"-no-merge=true",
			"-config=" + `{"SessionID":"update"}`,
		}
		ui := cli.NewMockUi()
		cmd := New(ui)
		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		readUpdate(t, methodName)
	})
	t.Run("config with no merge", func(t *testing.T) {
		methodName := createAuthMethod(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + methodName,
			"-no-merge=false",
			"-config=" + `{"SessionID":"update"}`,
		}
		ui := cli.NewMockUi()
		cmd := New(ui)
		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		readUpdate(t, methodName)
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
