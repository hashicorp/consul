// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/testrpc"
)

// TestLeader_PeeringSync_BidirectionalLargeSnapshot exercises two real servers
// that both export many services to each other before the peering is
// established. On connect both sides push their full snapshot at the same time,
// which used to deadlock the peering stream on HTTP/2 flow control: each side's
// stream handler blocked in Send() and therefore stopped reading, so neither
// side's flow-control window was ever replenished.
func TestLeader_PeeringSync_BidirectionalLargeSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	const numServices = 400

	ca := connect.TestCA(t, nil)
	_, acceptor := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "acceptor"
		c.Datacenter = "dc1"
		c.GRPCTLSPort = freeport.GetOne(t)
		c.CAConfig = &structs.CAConfiguration{
			ClusterID: connect.TestClusterID,
			Provider:  structs.ConsulCAProvider,
			Config: map[string]interface{}{
				"PrivateKey": ca.SigningKey,
				"RootCert":   ca.RootCert,
			},
		}
	})
	testrpc.WaitForLeader(t, acceptor.RPC, "dc1")

	_, dialer := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "dialer"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		c.PeeringEnabled = true
	})
	testrpc.WaitForLeader(t, dialer.RPC, "dc2")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	// Populate both servers before the peering exists so that both sides send
	// their full snapshot concurrently as soon as the stream comes up.
	populate := func(s *Server, prefix, consumerPeer string) {
		var out bool
		require.NoError(t, s.RPC(ctx, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Datacenter: s.config.Datacenter,
			Entry: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{{
					Name:      structs.WildcardSpecifier,
					Consumers: []structs.ServiceConsumer{{Peer: consumerPeer}},
				}},
			},
		}, &out))

		meta := map[string]string{}
		for k := 0; k < 4; k++ {
			meta[fmt.Sprintf("pad%d", k)] = strings.Repeat("x", 500)
		}
		for i := 0; i < numServices; i++ {
			name := fmt.Sprintf("%s-%04d", prefix, i)
			var regOut struct{}
			require.NoError(t, s.RPC(ctx, "Catalog.Register", &structs.RegisterRequest{
				Datacenter: s.config.Datacenter,
				Node:       prefix + "-node",
				Address:    "10.0.0.1",
				Service: &structs.NodeService{
					ID:      name,
					Service: name,
					Port:    8080,
					Meta:    meta,
				},
			}, &regOut))
		}
	}
	populate(acceptor, "acc", "dialer-peer")
	populate(dialer, "dial", "acceptor-peer")

	// Generate a token on the acceptor and establish from the dialer.
	acceptorConn, err := grpc.DialContext(ctx, acceptor.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(acceptor.config.RPCAddr.String())),
		//nolint:staticcheck
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = acceptorConn.Close() })

	tokenResp, err := pbpeering.NewPeeringServiceClient(acceptorConn).GenerateToken(ctx,
		&pbpeering.GenerateTokenRequest{PeerName: "dialer-peer"})
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(tokenResp.PeeringToken)
	require.NoError(t, err)
	var token structs.PeeringToken
	require.NoError(t, json.Unmarshal(tokenJSON, &token))

	dialerConn, err := grpc.DialContext(ctx, dialer.config.RPCAddr.String(),
		grpc.WithContextDialer(newServerDialer(dialer.config.RPCAddr.String())),
		//nolint:staticcheck
		grpc.WithInsecure(),
		grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = dialerConn.Close() })

	_, err = pbpeering.NewPeeringServiceClient(dialerConn).Establish(ctx, &pbpeering.EstablishRequest{
		PeerName:     "acceptor-peer",
		PeeringToken: tokenResp.PeeringToken,
	})
	require.NoError(t, err)

	_, dialerPeering, err := dialer.fsm.State().PeeringRead(nil, state.Query{Value: "acceptor-peer"})
	require.NoError(t, err)
	require.NotNil(t, dialerPeering)

	importedCount := func(s *Server, peerName string) int {
		_, services, err := s.fsm.State().ServiceList(nil, structs.DefaultEnterpriseMetaInDefaultPartition(), peerName)
		require.NoError(t, err)
		return len(services)
	}

	// Both catalogs must receive every service exported by the other side.
	deadline := time.Now().Add(90 * time.Second)
	var accImported, dialImported int
	for time.Now().Before(deadline) {
		accImported = importedCount(acceptor, "dialer-peer")
		dialImported = importedCount(dialer, "acceptor-peer")
		if accImported == numServices && dialImported == numServices {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if accImported == numServices && dialImported == numServices {
		accStatus, found := acceptor.peerStreamServer.StreamStatus(token.PeerID)
		require.True(t, found)
		require.True(t, accStatus.Connected)
		dialStatus, found := dialer.peerStreamServer.StreamStatus(dialerPeering.ID)
		require.True(t, found)
		require.True(t, dialStatus.Connected)
		return
	}

	var sb strings.Builder
	_ = pprof.Lookup("goroutine").WriteTo(&sb, 2)
	var stuck []string
	for _, g := range strings.Split(sb.String(), "\n\n") {
		if strings.Contains(g, "peerstream.(*Server).realHandleStream") {
			stuck = append(stuck, g)
		}
	}
	t.Fatalf("peering did not converge: acceptor imported %d/%d, dialer imported %d/%d\n\nrealHandleStream goroutines:\n%s",
		accImported, numServices, dialImported, numServices, strings.Join(stuck, "\n\n"))
}
