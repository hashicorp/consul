package test

import (
	"fmt"
	"os/exec"
	"testing"

	vapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

func TestDemo(t *testing.T) {
	vault := NewTestVaultServer(t, "vault", "local")
	defer vault.Stop()
	consul := NewTestConsulServer(t, "consul", "local")
	defer consul.Stop()

	t.Run("demo", func(t *testing.T) {
		demo(t, consul, vault)
	})
}

// Vault as a Consul Service Mesh Certificate Authority demo in code
func demo(t *testing.T, c TestConsulServer, v TestVaultServer) {
	// vault setup
	err := v.Client().Sys().Mount(rootPath+"/", &vapi.MountInput{Type: "pki"})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := v.Client().Sys().Unmount(rootPath + "/")
		require.NoError(t, err)
	})
	err = v.Client().Sys().Mount(intrPath+"/", &vapi.MountInput{Type: "pki"})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := v.Client().Sys().Unmount(intrPath + "/")
		require.NoError(t, err)
	})
	err = v.Client().Sys().PutPolicy(policyName, policyRules(rootPath, intrPath))
	require.NoError(t, err)
	t.Cleanup(func() {
		err := v.Client().Sys().DeletePolicy(policyName)
		require.NoError(t, err)
	})
	secret, err := v.Client().Auth().Token().Create(
		&vapi.TokenCreateRequest{Policies: []string{policyName}})
	token := secret.Auth.ClientToken
	t.Cleanup(func() {
		err := v.Client().Auth().Token().RevokeTree(token)
		require.NoError(t, err)
	})
	require.NoError(t, err)

	// consul setup
	_, err = c.Client().Connect().CASetConfig(
		caConf(v.Addr, token, rootPath, intrPath), nil)
	require.NoError(t, err)
	// can't undo this... maybe add note that new tests that touch this
	// will need to overwrite other setups

	// tests
	caconf, _, err := c.Client().Connect().CAGetConfig(nil)
	require.NoError(t, err)
	require.Equal(t, caconf.Provider, "vault")
	roots, _, err := c.Client().Agent().ConnectCARoots(nil)
	require.NoError(t, err)
	require.Len(t, roots.Roots, 2)
	leaf, _, err := c.Client().Agent().ConnectCALeaf(leafPath, nil)
	require.NoError(t, err)
	certpem1 := leaf.CertPEM
	require.Contains(t, certpem1, "CERTIFICATE")
	require.Contains(t, leaf.PrivateKeyPEM, "PRIVATE")
	leaf, _, err = c.Client().Agent().ConnectCALeaf(leafPath, nil)
	require.NoError(t, err)
	certpem2 := leaf.CertPEM
	require.Contains(t, certpem2, "CERTIFICATE")
	require.Contains(t, leaf.PrivateKeyPEM, "PRIVATE")
	require.Equal(t, certpem1, certpem2)

	// curlTests(t, c, v)
}

// tutorial's curl commands
func curlTests(t *testing.T, c TestConsulServer, v TestVaultServer) {
	out, err := exec.Command("curl", "-s", "-verbose", "--header",
		"X-Consul-Token: "+v.RootToken,
		c.HTTPAddr+"/v1/agent/connect/ca/leaf/leaf").CombinedOutput()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
	out, err = exec.Command("curl", "-s", "-verbose", "--header",
		"X-Consul-Token: "+v.RootToken,
		c.HTTPAddr+"/v1/agent/connect/ca/leaf/leaf").CombinedOutput()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
}

const (
	rootPath   = "connect_root"
	intrPath   = "connect_dc1_inter"
	leafPath   = "leaf"
	policyName = "ca"
)

func policyRules(rootPath, intrPath string) string {
	return fmt.Sprintf(policyRulesTmpl, rootPath, intrPath)
}

const policyRulesTmpl = `
path "/sys/mounts/%[1]s" {
  capabilities = [ "read" ]
}

path "/sys/mounts/%[2]s" {
  capabilities = [ "read" ]
}

path "/sys/mounts/%[2]s/tune" {
  capabilities = [ "update" ]
}

path "/%[1]s/*" {
  capabilities = [ "read", "update" ]
}

path "%[1]s/root/sign-intermediate" {
  capabilities = [ "update" ]
}

path "%[2]s/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "auth/token/renew-self" {
  capabilities = [ "update" ]
}

path "auth/token/lookup-self" {
  capabilities = [ "read" ]
}
`
