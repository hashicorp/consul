// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestOperator_RaftConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	body := bytes.NewBuffer(nil)
	req, _ := http.NewRequest("GET", "/v1/operator/raft/configuration", body)
	resp := httptest.NewRecorder()
	obj, err := a.srv.OperatorRaftConfiguration(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("bad code: %d", resp.Code)
	}
	out, ok := obj.(structs.RaftConfigurationResponse)
	if !ok {
		t.Fatalf("unexpected: %T", obj)
	}
	if len(out.Servers) != 1 ||
		!out.Servers[0].Leader ||
		!out.Servers[0].Voter {
		t.Fatalf("bad: %v", out)
	}
}

func TestOperator_RaftPeer(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("DELETE", "/v1/operator/raft/peer?address=nope", body)
		// If we get this error, it proves we sent the address all the
		// way through.
		resp := httptest.NewRecorder()
		_, err := a.srv.OperatorRaftPeer(resp, req)
		if err == nil || !strings.Contains(err.Error(),
			"address \"nope\" was not found in the Raft configuration") {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("DELETE", "/v1/operator/raft/peer?id=nope", body)
		// If we get this error, it proves we sent the ID all the
		// way through.
		resp := httptest.NewRecorder()
		_, err := a.srv.OperatorRaftPeer(resp, req)
		if err == nil || !strings.Contains(err.Error(),
			"id \"nope\" was not found in the Raft configuration") {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestOperator_KeyringInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	oldKey := "H3/9gBxcKKRf45CaI2DlRg=="
	newKey := "z90lFx3sZZLtTOkutXcwYg=="
	a := NewTestAgent(t, `
		encrypt = "`+oldKey+`"
	`)
	defer a.Shutdown()

	body := bytes.NewBufferString(fmt.Sprintf("{\"Key\":\"%s\"}", newKey))
	req, _ := http.NewRequest("POST", "/v1/operator/keyring", body)
	resp := httptest.NewRecorder()
	_, err := a.srv.OperatorKeyringEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	listResponse, err := a.ListKeys("", false, 0)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(listResponse.Responses) != 2 {
		t.Fatalf("bad: %d", len(listResponse.Responses))
	}

	for _, response := range listResponse.Responses {
		count, ok := response.Keys[newKey]
		if !ok {
			t.Fatalf("bad: %v", response.Keys)
		}
		if count != response.NumNodes {
			t.Fatalf("bad: %d, %d", count, response.NumNodes)
		}
	}
}

func TestOperator_KeyringList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	a := NewTestAgent(t, `
		encrypt = "`+key+`"
	`)
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/operator/keyring", nil)
	resp := httptest.NewRecorder()
	r, err := a.srv.OperatorKeyringEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	responses, ok := r.([]*structs.KeyringResponse)
	if !ok {
		t.Fatalf("err: %v", !ok)
	}

	// Check that we get both a LAN and WAN response, and that they both only
	// contain the original key
	if len(responses) != 2 {
		t.Fatalf("bad: %d", len(responses))
	}

	// WAN
	if len(responses[0].Keys) != 1 {
		t.Fatalf("bad: %d", len(responses[0].Keys))
	}
	if !responses[0].WAN {
		t.Fatalf("bad: %v", responses[0].WAN)
	}
	if _, ok := responses[0].Keys[key]; !ok {
		t.Fatalf("bad: %v", ok)
	}

	// LAN
	if len(responses[1].Keys) != 1 {
		t.Fatalf("bad: %d", len(responses[1].Keys))
	}
	if responses[1].WAN {
		t.Fatalf("bad: %v", responses[1].WAN)
	}
	if _, ok := responses[1].Keys[key]; !ok {
		t.Fatalf("bad: %v", ok)
	}
}
func TestOperator_KeyringListLocalOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	a := NewTestAgent(t, `
		encrypt = "`+key+`"
	`)
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/operator/keyring?local-only=1", nil)
	resp := httptest.NewRecorder()
	r, err := a.srv.OperatorKeyringEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	responses, ok := r.([]*structs.KeyringResponse)
	if !ok {
		t.Fatalf("err: %v", !ok)
	}

	// Check that we only get a LAN response with the original key
	if len(responses) != 1 {
		for _, r := range responses {
			fmt.Println(r)
		}
		t.Fatalf("bad: %d", len(responses))
	}

	// LAN
	if len(responses[0].Keys) != 1 {
		t.Fatalf("bad: %d", len(responses[1].Keys))
	}
	if responses[0].WAN {
		t.Fatalf("bad: %v", responses[1].WAN)
	}
	if _, ok := responses[0].Keys[key]; !ok {
		t.Fatalf("bad: %v", ok)
	}
}

func TestOperator_KeyringRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	tempKey := "z90lFx3sZZLtTOkutXcwYg=="
	a := NewTestAgent(t, `
		encrypt = "`+key+`"
	`)
	defer a.Shutdown()

	_, err := a.InstallKey(tempKey, "", 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the temp key is installed
	list, err := a.ListKeys("", false, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	responses := list.Responses
	if len(responses) != 2 {
		t.Fatalf("bad: %d", len(responses))
	}
	for _, response := range responses {
		if len(response.Keys) != 2 {
			t.Fatalf("bad: %d", len(response.Keys))
		}
		if _, ok := response.Keys[tempKey]; !ok {
			t.Fatalf("bad: %v", ok)
		}
	}

	body := bytes.NewBufferString(fmt.Sprintf("{\"Key\":\"%s\"}", tempKey))
	req, _ := http.NewRequest("DELETE", "/v1/operator/keyring", body)
	resp := httptest.NewRecorder()
	if _, err := a.srv.OperatorKeyringEndpoint(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the temp key has been removed
	list, err = a.ListKeys("", false, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	responses = list.Responses
	if len(responses) != 2 {
		t.Fatalf("bad: %d", len(responses))
	}
	for _, response := range responses {
		if len(response.Keys) != 1 {
			t.Fatalf("bad: %d", len(response.Keys))
		}
		if _, ok := response.Keys[tempKey]; ok {
			t.Fatalf("bad: %v", ok)
		}
	}
}

func TestOperator_KeyringUse(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	oldKey := "H3/9gBxcKKRf45CaI2DlRg=="
	newKey := "z90lFx3sZZLtTOkutXcwYg=="
	a := NewTestAgent(t, `
		encrypt = "`+oldKey+`"
	`)
	defer a.Shutdown()

	if _, err := a.InstallKey(newKey, "", 0); err != nil {
		t.Fatalf("err: %v", err)
	}

	body := bytes.NewBufferString(fmt.Sprintf("{\"Key\":\"%s\"}", newKey))
	req, _ := http.NewRequest("PUT", "/v1/operator/keyring", body)
	resp := httptest.NewRecorder()
	_, err := a.srv.OperatorKeyringEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if _, err := a.RemoveKey(oldKey, "", 0); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure only the new key remains
	list, err := a.ListKeys("", false, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	responses := list.Responses
	if len(responses) != 2 {
		t.Fatalf("bad: %d", len(responses))
	}
	for _, response := range responses {
		if len(response.Keys) != 1 {
			t.Fatalf("bad: %d", len(response.Keys))
		}
		if _, ok := response.Keys[newKey]; !ok {
			t.Fatalf("bad: %v", ok)
		}
	}
}

func TestOperator_Keyring_InvalidRelayFactor(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	a := NewTestAgent(t, `
		encrypt = "`+key+`"
	`)
	defer a.Shutdown()

	cases := map[string]string{
		"999":  "Relay factor must be in range",
		"asdf": "Error parsing relay factor",
	}
	for relayFactor, errString := range cases {
		req, err := http.NewRequest("GET", "/v1/operator/keyring?relay-factor="+relayFactor, nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		_, err = a.srv.OperatorKeyringEndpoint(resp, req)
		require.Error(t, err, "tc: "+relayFactor)
		require.Contains(t, err.Error(), errString, "tc: "+relayFactor)
	}
}

func TestOperator_Keyring_LocalOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	a := NewTestAgent(t, `
		encrypt = "`+key+`"
	`)
	defer a.Shutdown()

	cases := []struct {
		description string
		method      string
		local       interface{}
		ok          bool
	}{
		{"all ok", "GET", true, true},
		{"garbage local-only value", "GET", "garbage", false},
		{"wrong method (DELETE)", "DELETE", true, false},
	}

	for _, tc := range cases {
		url := fmt.Sprintf("/v1/operator/keyring?local-only=%v", tc.local)
		req, err := http.NewRequest(tc.method, url, nil)
		require.NoError(t, err, "tc: "+tc.description)

		resp := httptest.NewRecorder()
		_, err = a.srv.OperatorKeyringEndpoint(resp, req)
		if tc.ok {
			require.NoError(t, err, "tc: "+tc.description)
		}
		if !tc.ok {
			require.Error(t, err, "tc: "+tc.description)
		}
	}
}

func TestOperator_AutopilotGetConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	body := bytes.NewBuffer(nil)
	req, _ := http.NewRequest("GET", "/v1/operator/autopilot/configuration", body)
	resp := httptest.NewRecorder()
	obj, err := a.srv.OperatorAutopilotConfiguration(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("bad code: %d", resp.Code)
	}
	out, ok := obj.(api.AutopilotConfiguration)
	if !ok {
		t.Fatalf("unexpected: %T", obj)
	}
	if !out.CleanupDeadServers {
		t.Fatalf("bad: %#v", out)
	}
}

func TestOperator_AutopilotSetConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Provide a non-default value only for CleanupDeadServers.
	// Expect all other fields to be updated with default values
	// (except CreateIndex and ModifyIndex).
	body := bytes.NewBuffer([]byte(`{"CleanupDeadServers": false}`))
	expected := structs.AutopilotConfig{
		CleanupDeadServers:      false, // only non-default value
		LastContactThreshold:    200 * time.Millisecond,
		MaxTrailingLogs:         250,
		MinQuorum:               0,
		ServerStabilizationTime: 10 * time.Second,
		RedundancyZoneTag:       "",
		DisableUpgradeMigration: false,
		UpgradeVersionTag:       "",
	}

	req, _ := http.NewRequest("PUT", "/v1/operator/autopilot/configuration", body)
	resp := httptest.NewRecorder()
	if _, err := a.srv.OperatorAutopilotConfiguration(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("bad code: %d", resp.Code)
	}

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	var reply structs.AutopilotConfig
	if err := a.RPC(context.Background(), "Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// For equality comparison check, ignore CreateIndex and ModifyIndex
	expected.CreateIndex = reply.CreateIndex
	expected.ModifyIndex = reply.ModifyIndex
	require.Equal(t, expected, reply)
}

func TestOperator_AutopilotCASConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	body := bytes.NewBuffer([]byte(`{"CleanupDeadServers": false}`))
	req, _ := http.NewRequest("PUT", "/v1/operator/autopilot/configuration", body)
	resp := httptest.NewRecorder()
	if _, err := a.srv.OperatorAutopilotConfiguration(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("bad code: %d", resp.Code)
	}

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	var reply structs.AutopilotConfig
	if err := a.RPC(context.Background(), "Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	if reply.CleanupDeadServers {
		t.Fatalf("bad: %#v", reply)
	}

	// Create a CAS request, bad index
	{
		buf := bytes.NewBuffer([]byte(`{"CleanupDeadServers": true}`))
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/operator/autopilot/configuration?cas=%d", reply.ModifyIndex-1), buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.OperatorAutopilotConfiguration(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); res {
			t.Fatalf("should NOT work")
		}
	}

	// Create a CAS request, good index
	{
		buf := bytes.NewBuffer([]byte(`{"CleanupDeadServers": true}`))
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/operator/autopilot/configuration?cas=%d", reply.ModifyIndex), buf)
		resp := httptest.NewRecorder()
		obj, err := a.srv.OperatorAutopilotConfiguration(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}
	}

	// Verify the update
	if err := a.RPC(context.Background(), "Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reply.CleanupDeadServers {
		t.Fatalf("bad: %#v", reply)
	}
}

func TestOperator_ServerHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		raft_protocol = 3
	`)
	defer a.Shutdown()

	body := bytes.NewBuffer(nil)
	req, _ := http.NewRequest("GET", "/v1/operator/autopilot/health", body)
	retry.Run(t, func(r *retry.R) {
		resp := httptest.NewRecorder()
		obj, err := a.srv.OperatorServerHealth(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			r.Fatalf("bad code: %d", resp.Code)
		}
		out, ok := obj.(*api.OperatorHealthReply)
		if !ok {
			r.Fatalf("unexpected: %T", obj)
		}
		if len(out.Servers) != 1 ||
			!out.Servers[0].Healthy ||
			out.Servers[0].Name != a.Config.NodeName ||
			out.Servers[0].SerfStatus != "alive" ||
			out.FailureTolerance != 0 {
			r.Fatalf("bad: %v", out)
		}
	})
}

func TestOperator_ServerHealth_Unhealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		raft_protocol = 3
		autopilot {
			last_contact_threshold = "-1s"
		}
	`)
	defer a.Shutdown()

	body := bytes.NewBuffer(nil)
	req, _ := http.NewRequest("GET", "/v1/operator/autopilot/health", body)
	retry.Run(t, func(r *retry.R) {
		resp := httptest.NewRecorder()
		obj, err := a.srv.OperatorServerHealth(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if resp.Code != 429 {
			r.Fatalf("bad code: %d", resp.Code)
		}
		out, ok := obj.(*api.OperatorHealthReply)
		if !ok {
			r.Fatalf("unexpected: %T", obj)
		}
		if len(out.Servers) != 1 ||
			out.Healthy ||
			out.Servers[0].Name != a.Config.NodeName {
			r.Fatalf("bad: %#v", out.Servers)
		}
	})
}

func TestOperator_AutopilotState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	req, err := http.NewRequest("GET", "/v1/operator/autopilot/state", nil)
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		resp := httptest.NewRecorder()
		obj, err := a.srv.OperatorAutopilotState(resp, req)
		require.NoError(r, err)
		require.Equal(r, 200, resp.Code)
		state, ok := obj.(*api.AutopilotState)
		require.True(r, ok)

		srv, ok := state.Servers[string(a.config.NodeID)]
		require.True(r, ok)
		require.True(r, srv.Healthy)
		require.Equal(r, a.config.NodeName, srv.Name)

	})
}

func TestAutopilotStateToAPIConversion(t *testing.T) {
	var leaderID raft.ServerID = "79324811-9588-4311-b208-f272e38aaabf"
	var follower1ID raft.ServerID = "ef8aee9a-f9d6-4ec4-b383-aac956bdb80f"
	var follower2ID raft.ServerID = "ae84aefb-a303-4734-8739-5c102d4ee2d9"
	input := autopilot.State{
		Healthy:          true,
		FailureTolerance: 1,
		Leader:           leaderID,
		Voters: []raft.ServerID{
			leaderID,
			follower1ID,
			follower2ID,
		},
		Servers: map[raft.ServerID]*autopilot.ServerState{
			leaderID: {
				Server: autopilot.Server{
					ID:         leaderID,
					Name:       "node1",
					Address:    "198.18.0.1:8300",
					NodeStatus: autopilot.NodeAlive,
					Version:    "1.9.0",
					Meta: map[string]string{
						"foo": "bar",
					},
					NodeType: autopilot.NodeVoter,
				},
				State: autopilot.RaftLeader,
				Stats: autopilot.ServerStats{
					LastContact: 0,
					LastTerm:    3,
					LastIndex:   42,
				},
				Health: autopilot.ServerHealth{
					Healthy:     true,
					StableSince: time.Date(2020, 11, 6, 14, 51, 0, 0, time.UTC),
				},
			},
			follower1ID: {
				Server: autopilot.Server{
					ID:         follower1ID,
					Name:       "node2",
					Address:    "198.18.0.2:8300",
					NodeStatus: autopilot.NodeAlive,
					Version:    "1.9.0",
					Meta: map[string]string{
						"bar": "baz",
					},
					NodeType: autopilot.NodeVoter,
				},
				State: autopilot.RaftVoter,
				Stats: autopilot.ServerStats{
					LastContact: time.Millisecond,
					LastTerm:    3,
					LastIndex:   41,
				},
				Health: autopilot.ServerHealth{
					Healthy:     true,
					StableSince: time.Date(2020, 11, 6, 14, 52, 0, 0, time.UTC),
				},
			},
			follower2ID: {
				Server: autopilot.Server{
					ID:         follower2ID,
					Name:       "node3",
					Address:    "198.18.0.3:8300",
					NodeStatus: autopilot.NodeAlive,
					Version:    "1.9.0",
					Meta: map[string]string{
						"baz": "foo",
					},
					NodeType: autopilot.NodeVoter,
				},
				State: autopilot.RaftVoter,
				Stats: autopilot.ServerStats{
					LastContact: 2 * time.Millisecond,
					LastTerm:    3,
					LastIndex:   39,
				},
				Health: autopilot.ServerHealth{
					Healthy:     true,
					StableSince: time.Date(2020, 11, 6, 14, 53, 0, 0, time.UTC),
				},
			},
		},
	}

	expected := api.AutopilotState{
		Healthy:                    true,
		FailureTolerance:           1,
		OptimisticFailureTolerance: 1,
		Leader:                     string(leaderID),
		Voters: []string{
			string(leaderID),
			string(follower1ID),
			string(follower2ID),
		},
		Servers: map[string]api.AutopilotServer{
			string(leaderID): {
				ID:         string(leaderID),
				Name:       "node1",
				Address:    "198.18.0.1:8300",
				NodeStatus: "alive",
				Version:    "1.9.0",
				Meta: map[string]string{
					"foo": "bar",
				},
				NodeType:    api.AutopilotTypeVoter,
				Status:      api.AutopilotServerLeader,
				LastContact: api.NewReadableDuration(0),
				LastTerm:    3,
				LastIndex:   42,
				Healthy:     true,
				StableSince: time.Date(2020, 11, 6, 14, 51, 0, 0, time.UTC),
			},
			string(follower1ID): {
				ID:         string(follower1ID),
				Name:       "node2",
				Address:    "198.18.0.2:8300",
				NodeStatus: "alive",
				Version:    "1.9.0",
				Meta: map[string]string{
					"bar": "baz",
				},
				NodeType:    api.AutopilotTypeVoter,
				Status:      api.AutopilotServerVoter,
				LastContact: api.NewReadableDuration(time.Millisecond),
				LastTerm:    3,
				LastIndex:   41,
				Healthy:     true,
				StableSince: time.Date(2020, 11, 6, 14, 52, 0, 0, time.UTC),
			},
			string(follower2ID): {
				ID:         string(follower2ID),
				Name:       "node3",
				Address:    "198.18.0.3:8300",
				NodeStatus: "alive",
				Version:    "1.9.0",
				Meta: map[string]string{
					"baz": "foo",
				},
				NodeType:    api.AutopilotTypeVoter,
				Status:      api.AutopilotServerVoter,
				LastContact: api.NewReadableDuration(2 * time.Millisecond),
				LastTerm:    3,
				LastIndex:   39,
				Healthy:     true,
				StableSince: time.Date(2020, 11, 6, 14, 53, 0, 0, time.UTC),
			},
		},
	}

	require.Equal(t, &expected, autopilotToAPIState(&input))
}
