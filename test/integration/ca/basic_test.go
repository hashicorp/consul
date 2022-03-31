package ca

import (
	"fmt"
	"testing"

	vaultcontainer "github.com/hashicorp/consul/integration/ca/libs/vault-node"

	consulcontainer "github.com/hashicorp/consul/integration/ca/libs/consul-node"

	"github.com/stretchr/testify/require"
)

const (
	connectCAPolicyTemplate = `
path "/sys/mounts" {
  capabilities = [ "read" ]
}

path "/sys/mounts/connect_root" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "/sys/mounts/%s/connect_inter" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "/connect_root/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "/%s/connect_inter/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}
`
	caPolicy = `
path "pki/cert/ca" {
  capabilities = ["read"]
}`
)

func TestBasic(t *testing.T) {
	consulNode, err := consulcontainer.NewNode()
	require.NoError(t, err)
	leader, err := consulNode.Client.Status().Leader()
	require.NoError(t, err)
	require.NotEmpty(t, leader)
}

func TestBasicWithConfig(t *testing.T) {
	vaultNode, err := vaultcontainer.NewNode()
	require.NoError(t, err)
	response, err := vaultNode.Client.Sys().Health()
	require.NoError(t, err)
	require.True(t, response.Initialized)
	require.False(t, response.Sealed)

	const datacenter = "dc1"
	err = vaultNode.Client.Sys().PutPolicy(
		fmt.Sprintf("connect-ca-%s", datacenter),
		fmt.Sprintf(connectCAPolicyTemplate, datacenter, datacenter))
	require.NoError(t, err)

	params := map[string]interface{}{
		"common_name": "Consul CA",
		"ttl":         "24h",
	}
	_, err = vaultNode.Client.Logical().Write("pki/root/generate/internal", params)
	require.NoError(t, err)

	err = vaultNode.Client.Sys().PutPolicy("consul-ca", caPolicy)
	require.NoError(t, err)
	// Create the Vault PKI Role.
	consulServerDNSName := "test" + "-consul-server"
	allowedDomains := fmt.Sprintf("%s.consul,%s,%s.%s,%s.%s.svc", datacenter, consulServerDNSName, consulServerDNSName, "", consulServerDNSName, "")
	params = map[string]interface{}{
		"allowed_domains":    allowedDomains,
		"allow_bare_domains": "true",
		"allow_localhost":    "true",
		"allow_subdomains":   "true",
		"generate_lease":     "true",
		"max_ttl":            "1h",
	}

	pkiRoleName := fmt.Sprintf("consul-server-%s", datacenter)

	_, err = vaultNode.Client.Logical().Write(fmt.Sprintf("pki/roles/%s", pkiRoleName), params)
	require.NoError(t, err)

	certificateIssuePath := fmt.Sprintf("pki/issue/%s", pkiRoleName)
	serverTLSPolicy := fmt.Sprintf(`
path %q {
  capabilities = ["create", "update"]
}`, certificateIssuePath)

	// Create the server policy.
	err = vaultNode.Client.Sys().PutPolicy(pkiRoleName, serverTLSPolicy)
	require.NoError(t, err)

	/*consulNode, err := consulcontainer.NewNodeWitConfig(context.Background(),
			`node_name="dc1-consul-client1"
					log_level="TRACE"
			        verify_incoming=true
	    			verify_outgoing=true
	   				verify_server_hostname=true
			`)
		require.NoError(t, err)
		leader, err := consulNode.Client.Status().Leader()
		require.NoError(t, err)
		require.NotEmpty(t, leader)*/
}
