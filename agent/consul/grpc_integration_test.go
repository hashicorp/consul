// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/proto-public/pbacl"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
	"github.com/hashicorp/consul/proto-public/pbserverdiscovery"
)

func TestGRPCIntegration_ConnectCA_Sign(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// The gRPC endpoint itself well-tested with mocks. This test checks we're
	// correctly wiring everything up in the server by:
	//
	//	* Starting a cluster with multiple servers.
	//	* Making a request to a follower's external gRPC port.
	//	* Ensuring that the request is correctly forwarded to the leader.
	//	* Ensuring we get a valid certificate back (so it went through the CAManager).
	server1, conn1, _ := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = false
		c.BootstrapExpect = 2
	})

	server2, conn2, _ := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = false
	})

	joinLAN(t, server2, server1)

	waitForLeaderEstablishment(t, server1, server2)

	conn := conn2
	if server2.IsLeader() {
		conn = conn1
	}

	client := pbconnectca.NewConnectCAServiceClient(conn)

	csr, _ := connect.TestCSR(t, &connect.SpiffeIDService{
		Host:       connect.TestClusterID + ".consul",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	options := structs.QueryOptions{Token: TestDefaultInitialManagementToken}
	ctx, err := external.ContextWithQueryOptions(ctx, options)
	require.NoError(t, err)

	// This would fail if it wasn't forwarded to the leader.
	rsp, err := client.Sign(ctx, &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.NoError(t, err)

	_, err = connect.ParseCert(rsp.CertPem)
	require.NoError(t, err)
}

func TestGRPCIntegration_ServerDiscovery_WatchServers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// The gRPC endpoint itself well-tested with mocks. This test checks we're
	// correctly wiring everything up in the server by:
	//
	//	* Starting a server
	// * Initiating the gRPC stream
	// * Validating the snapshot
	// * Adding another server
	// * Validating another message is sent.

	server1, conn, _ := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = true
		c.BootstrapExpect = 1
	})
	waitForLeaderEstablishment(t, server1)

	client := pbserverdiscovery.NewServerDiscoveryServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	options := structs.QueryOptions{Token: TestDefaultInitialManagementToken}
	ctx, err := external.ContextWithQueryOptions(ctx, options)
	require.NoError(t, err)

	serverStream, err := client.WatchServers(ctx, &pbserverdiscovery.WatchServersRequest{Wan: false})
	require.NoError(t, err)

	rsp, err := serverStream.Recv()
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.Len(t, rsp.Servers, 1)

	_, server2, _ := testACLServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	}, false)

	// join the new server to the leader
	joinLAN(t, server2, server1)

	// now receive the event containing 2 servers
	rsp, err = serverStream.Recv()
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.Len(t, rsp.Servers, 2)
}

func TestGRPCIntegration_ACL_Login_Logout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// The gRPC endpoints themselves are well unit tested - this test ensures we're
	// correctly wiring everything up and exercises the cross-dc RPC forwarding by:
	//
	//	* Starting two servers in different datacenters.
	//	* WAN federating them.
	//	* Configuring ACL token replication in the secondary datacenter.
	//	* Registering an auth method (configured for global tokens) in the primary
	//	  datacenter.
	//	* Making a Login request to the secondary DC, with the request's Datacenter
	//	  field set to "primary" (to exercise user requested DC forwarding).
	//	* Waiting for the token to be replicated to the secondary DC.
	//	* Making a Logout request to the secondary DC, with the request's Datacenter
	//	  field set to "secondary" — the request will be forwarded to the primary
	//	  datacenter anyway because the token is global.

	// Start the primary DC.
	primary, _, primaryCodec := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = true
		c.BootstrapExpect = 1
		c.Datacenter = "primary"
		c.PrimaryDatacenter = "primary"
	})
	waitForLeaderEstablishment(t, primary)

	// Configured the auth method.
	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)
	testauth.InstallSessionToken(testSessionID, "fake-token", "default", "demo", "abc123")

	authMethod, err := upsertTestCustomizedAuthMethod(primaryCodec, TestDefaultInitialManagementToken, "primary", func(method *structs.ACLAuthMethod) {
		method.Config = map[string]interface{}{
			"SessionID": testSessionID,
		}
		method.TokenLocality = "global"
	})
	require.NoError(t, err)

	_, err = upsertTestBindingRule(primaryCodec, TestDefaultInitialManagementToken, "primary", authMethod.Name, "", structs.BindingRuleBindTypeService, "demo")
	require.NoError(t, err)

	// Start the secondary DC.
	secondary, secondaryConn, _ := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = true
		c.BootstrapExpect = 1
		c.Datacenter = "secondary"
		c.PrimaryDatacenter = "primary"
		c.ACLTokenReplication = true
	})
	secondary.tokens.UpdateReplicationToken(TestDefaultInitialManagementToken, tokenStore.TokenSourceConfig)
	waitForLeaderEstablishment(t, secondary)

	// WAN federate the primary and secondary DCs.
	joinWAN(t, primary, secondary)

	client := pbacl.NewACLServiceClient(secondaryConn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// Make a Login request to the secondary DC, but request that it is forwarded
	// to the primary DC.
	rsp, err := client.Login(ctx, &pbacl.LoginRequest{
		AuthMethod:  authMethod.Name,
		BearerToken: "fake-token",
		Datacenter:  "primary",
	})
	require.NoError(t, err)
	require.NotNil(t, rsp.Token)
	require.NotEmpty(t, rsp.Token.AccessorId)
	require.NotEmpty(t, rsp.Token.SecretId)

	// Check token was created in the primary DC.
	tokenIdx, token, err := primary.FSM().State().ACLTokenGetByAccessor(nil, rsp.Token.AccessorId, nil)
	require.NoError(t, err)
	require.NotNil(t, token)
	require.False(t, token.Local, "token should be global")

	// Wait for token to be replicated to the secondary DC.
	waitForNewACLReplication(t, secondary, structs.ACLReplicateTokens, 0, tokenIdx, 0)

	// Make a Logout request to the secondary DC, the request should be forwarded
	// to the primary DC anyway because the token is global.
	_, err = client.Logout(ctx, &pbacl.LogoutRequest{
		Token:      rsp.Token.SecretId,
		Datacenter: "secondary",
	})
	require.NoError(t, err)
}
