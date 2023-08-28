// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/consul/internal/mesh"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestAgent_local_proxycfg(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	token := generateUUID()

	svc := &structs.NodeService{
		ID:             "db",
		Service:        "db",
		Port:           5000,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.NoError(t, a.State.AddServiceWithChecks(svc, nil, token, true))

	proxy := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "db-sidecar-proxy",
		Service: "db-sidecar-proxy",
		Port:    5000,
		// Set this internal state that we expect sidecar registrations to have.
		LocallyRegisteredAsSidecar: true,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "db",
			Upstreams:              structs.TestUpstreams(t, false),
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.NoError(t, a.State.AddServiceWithChecks(proxy, nil, token, true))

	// This is a little gross, but this gives us the layered pair of
	// local/catalog sources for now.
	cfg := a.xdsServer.CfgSrc

	var (
		timer      = time.After(100 * time.Millisecond)
		timerFired = false
		finalTimer <-chan time.Time
	)

	var (
		firstTime = true
		ch        <-chan proxycfg.ProxySnapshot
		stc       limiter.SessionTerminatedChan
		cancel    proxycfg.CancelFunc
	)
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()
	for {
		if ch == nil {
			// Sign up for a stream of config snapshots, in the same manner as the xds server.
			sid := proxy.CompoundServiceID()

			if firstTime {
				firstTime = false
			} else {
				t.Logf("re-creating watch")
			}

			// Prior to fixes in https://github.com/hashicorp/consul/pull/16497
			// this call to Watch() would deadlock.
			var err error
			ch, stc, cancel, err = cfg.Watch(rtest.Resource(mesh.ProxyConfigurationType, sid.ID, registry).ID(), a.config.NodeName, token)
			require.NoError(t, err)
		}
		select {
		case <-stc:
			t.Fatal("session unexpectedly terminated")
		case snap, ok := <-ch:
			if !ok {
				t.Logf("channel is closed")
				cancel()
				ch, stc, cancel = nil, nil, nil
				continue
			}
			require.NotNil(t, snap)
			if !timerFired {
				t.Fatal("should not have gotten snapshot until after we manifested the token")
			}
			return
		case <-timer:
			timerFired = true
			finalTimer = time.After(1 * time.Second)

			// This simulates the eventual consistency of a token
			// showing up on a server after it's creation by
			// pre-creating the UUID and later using that as the
			// initial SecretID for a real token.
			gotToken := testWriteToken(t, a, &api.ACLToken{
				AccessorID:  generateUUID(),
				SecretID:    token,
				Description: "my token",
				ServiceIdentities: []*api.ACLServiceIdentity{{
					ServiceName: "db",
				}},
			})
			require.Equal(t, token, gotToken)
		case <-finalTimer:
			t.Fatal("did not receive a snapshot after the token manifested")
		}
	}

}

func testWriteToken(t *testing.T, a *TestAgent, tok *api.ACLToken) string {
	req, _ := http.NewRequest("PUT", "/v1/acl/token", jsonReader(tok))
	req.Header.Add("X-Consul-Token", "root")
	resp := httptest.NewRecorder()
	a.srv.h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	dec := json.NewDecoder(resp.Body)
	aclResp := &structs.ACLToken{}
	require.NoError(t, dec.Decode(aclResp))
	return aclResp.SecretID
}
