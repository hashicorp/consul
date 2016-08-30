package agent

import (
	"bytes"
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
