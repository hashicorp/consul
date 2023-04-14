package test

import (
	"fmt"
	"os/exec"
	"testing"

	vapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

const (
	rootPath   = "root"
	intrPath   = "intermediate"
	leafPath   = "leaf"
	policyName = "ca"
)

func TestMatrix(t *testing.T) {
	// for version-matrix
	// run consul version
	// run vault version
	// test..
	//     start vault/consul
	vault := NewTestVaultServer(t, "vault")
	defer vault.Stop()
	consul := NewTestConsulServer(t, "consul")
	defer consul.Stop()
	//     config vault/consul
	setup := func(t *testing.T, v TestVaultServer, c TestConsulServer) {
		// vault
		err := v.Client().Sys().Mount(rootPath+"/", &vapi.MountInput{Type: "pki"})
		require.NoError(t, err)
		err = v.Client().Sys().Mount(intrPath+"/", &vapi.MountInput{Type: "pki"})
		require.NoError(t, err)
		err = v.Client().Sys().PutPolicy(policyName, policyRules)
		require.NoError(t, err)
		secret, err := v.Client().Auth().Token().Create(
			&vapi.TokenCreateRequest{Policies: []string{"ca"}})
		token := secret.Auth.ClientToken
		_ = token
		require.NoError(t, err)
		// consul
		// XXX why can't I use `token` here???
		_, err = c.Client().Connect().CASetConfig(
			caConf(v.Addr, v.RootToken), nil)
		require.NoError(t, err)
		caconf, _, err := c.Client().Connect().CAGetConfig(nil)
		require.NoError(t, err)
		fmt.Printf("%#v\n\n", caconf)
		roots, _, err := c.Client().Agent().ConnectCARoots(nil)
		require.NoError(t, err)
		fmt.Printf("%#v\n\n", roots.Roots[0])
		leaf, md, err := c.Client().Agent().ConnectCALeaf(leafPath, nil)
		require.NoError(t, err)
		fmt.Printf("%#v\n\n", leaf)
		fmt.Printf("%#v\n\n", md)
		leaf, md, err = c.Client().Agent().ConnectCALeaf(leafPath, nil)
		require.NoError(t, err)
		fmt.Printf("%#v\n\n", md)
		fmt.Printf("%#v\n\n", leaf)
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
	setup(t, vault, consul)
	//     run tests
	//     cleanup vault
	//     cleanup consul
}

const policyRules = `
path "/sys/mounts/connect_root" {
  capabilities = [ "read" ]
}

path "/sys/mounts/connect_dc1_inter" {
  capabilities = [ "read" ]
}

path "/sys/mounts/connect_dc1_inter/tune" {
  capabilities = [ "update" ]
}

path "/connect_root/" {
  capabilities = [ "read" ]
}

path "/connect_root/root/sign-intermediate" {
  capabilities = [ "update" ]
}

path "/connect_dc1_inter/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "auth/token/renew-self" {
  capabilities = [ "update" ]
}

path "auth/token/lookup-self" {
  capabilities = [ "read" ]
}
`
