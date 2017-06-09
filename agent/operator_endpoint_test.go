package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
)

func TestOperator_RaftConfiguration(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
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
	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t.Name(), nil)
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
		a := NewTestAgent(t.Name(), nil)
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
	t.Parallel()
	oldKey := "H3/9gBxcKKRf45CaI2DlRg=="
	newKey := "z90lFx3sZZLtTOkutXcwYg=="
	cfg := TestConfig()
	cfg.EncryptKey = oldKey
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	body := bytes.NewBufferString(fmt.Sprintf("{\"Key\":\"%s\"}", newKey))
	req, _ := http.NewRequest("POST", "/v1/operator/keyring", body)
	resp := httptest.NewRecorder()
	_, err := a.srv.OperatorKeyringEndpoint(resp, req)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	listResponse, err := a.ListKeys("", 0)
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
	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	cfg := TestConfig()
	cfg.EncryptKey = key
	a := NewTestAgent(t.Name(), cfg)
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

func TestOperator_KeyringRemove(t *testing.T) {
	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	tempKey := "z90lFx3sZZLtTOkutXcwYg=="
	cfg := TestConfig()
	cfg.EncryptKey = key
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	_, err := a.InstallKey(tempKey, "", 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the temp key is installed
	list, err := a.ListKeys("", 0)
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
	list, err = a.ListKeys("", 0)
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
	t.Parallel()
	oldKey := "H3/9gBxcKKRf45CaI2DlRg=="
	newKey := "z90lFx3sZZLtTOkutXcwYg=="
	cfg := TestConfig()
	cfg.EncryptKey = oldKey
	a := NewTestAgent(t.Name(), cfg)
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
	list, err := a.ListKeys("", 0)
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
	t.Parallel()
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	cfg := TestConfig()
	cfg.EncryptKey = key
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	cases := map[string]string{
		"999":  "Relay factor must be in range",
		"asdf": "Error parsing relay factor",
	}
	for relayFactor, errString := range cases {
		req, _ := http.NewRequest("GET", "/v1/operator/keyring?relay-factor="+relayFactor, nil)
		resp := httptest.NewRecorder()
		_, err := a.srv.OperatorKeyringEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		body := resp.Body.String()
		if !strings.Contains(body, errString) {
			t.Fatalf("bad: %v", body)
		}
	}
}

func TestOperator_AutopilotGetConfiguration(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	if err := a.RPC("Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply.CleanupDeadServers {
		t.Fatalf("bad: %#v", reply)
	}
}

func TestOperator_AutopilotCASConfiguration(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	if err := a.RPC("Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
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
	if err := a.RPC("Operator.AutopilotGetConfiguration", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reply.CleanupDeadServers {
		t.Fatalf("bad: %#v", reply)
	}
}

func TestOperator_ServerHealth(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.RaftProtocol = 3
	a := NewTestAgent(t.Name(), cfg)
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
	t.Parallel()
	cfg := TestConfig()
	cfg.RaftProtocol = 3
	threshold := time.Duration(-1)
	cfg.Autopilot.LastContactThreshold = &threshold
	a := NewTestAgent(t.Name(), cfg)
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
