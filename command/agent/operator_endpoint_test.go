package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

func TestOperator_OperatorRaftConfiguration(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest("GET", "/v1/operator/raft/configuration", body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		obj, err := srv.OperatorRaftConfiguration(resp, req)
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
	})
}

func TestOperator_OperatorRaftPeer(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest("DELETE", "/v1/operator/raft/peer?address=nope", body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// If we get this error, it proves we sent the address all the
		// way through.
		resp := httptest.NewRecorder()
		_, err = srv.OperatorRaftPeer(resp, req)
		if err == nil || !strings.Contains(err.Error(),
			"address \"nope\" was not found in the Raft configuration") {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestOperator_KeyringInstall(t *testing.T) {
	oldKey := "H3/9gBxcKKRf45CaI2DlRg=="
	newKey := "z90lFx3sZZLtTOkutXcwYg=="
	configFunc := func(c *Config) {
		c.EncryptKey = oldKey
	}
	httpTestWithConfig(t, func(srv *HTTPServer) {
		body := bytes.NewBufferString(fmt.Sprintf("{\"Key\":\"%s\"}", newKey))
		req, err := http.NewRequest("PUT", "/v1/operator/keyring/install", body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		_, err = srv.OperatorKeyringInstall(resp, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		listResponse, err := srv.agent.ListKeys("")
		if err != nil {
			t.Fatalf("err: %s", err)
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
	}, configFunc)
}

func TestOperator_KeyringList(t *testing.T) {
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	configFunc := func(c *Config) {
		c.EncryptKey = key
	}
	httpTestWithConfig(t, func(srv *HTTPServer) {
		req, err := http.NewRequest("GET", "/v1/operator/keyring/list", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		r, err := srv.OperatorKeyringList(resp, req)
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
		for _, response := range responses {
			if len(response.Keys) != 1 {
				t.Fatalf("bad: %d", len(response.Keys))
			}
			if _, ok := response.Keys[key]; !ok {
				t.Fatalf("bad: %v", ok)
			}
		}
	}, configFunc)
}

func TestOperator_KeyringRemove(t *testing.T) {
	key := "H3/9gBxcKKRf45CaI2DlRg=="
	tempKey := "z90lFx3sZZLtTOkutXcwYg=="
	configFunc := func(c *Config) {
		c.EncryptKey = key
	}
	httpTestWithConfig(t, func(srv *HTTPServer) {
		_, err := srv.agent.InstallKey(tempKey, "")
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make sure the temp key is installed
		list, err := srv.agent.ListKeys("")
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
		req, err := http.NewRequest("DELETE", "/v1/operator/keyring/remove", body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		_, err = srv.OperatorKeyringRemove(resp, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Make sure the temp key has been removed
		list, err = srv.agent.ListKeys("")
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
	}, configFunc)
}

func TestOperator_KeyringUse(t *testing.T) {
	oldKey := "H3/9gBxcKKRf45CaI2DlRg=="
	newKey := "z90lFx3sZZLtTOkutXcwYg=="
	configFunc := func(c *Config) {
		c.EncryptKey = oldKey
	}
	httpTestWithConfig(t, func(srv *HTTPServer) {
		if _, err := srv.agent.InstallKey(newKey, ""); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBufferString(fmt.Sprintf("{\"Key\":\"%s\"}", newKey))
		req, err := http.NewRequest("PUT", "/v1/operator/keyring/use", body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		_, err = srv.OperatorKeyringUse(resp, req)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if _, err := srv.agent.RemoveKey(oldKey, ""); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make sure only the new key remains
		list, err := srv.agent.ListKeys("")
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
	}, configFunc)
}
