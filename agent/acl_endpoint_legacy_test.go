package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestACL_Disabled_Response(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	tests := []func(resp http.ResponseWriter, req *http.Request) (interface{}, error){
		a.srv.ACLBootstrap,
		a.srv.ACLDestroy,
		a.srv.ACLCreate,
		a.srv.ACLUpdate,
		a.srv.ACLClone,
		a.srv.ACLGet,
		a.srv.ACLList,
		a.srv.ACLReplicationStatus,
		a.srv.AgentToken, // See TestAgent_Token.
	}
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/should/not/care", nil)
			resp := httptest.NewRecorder()
			obj, err := tt(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if obj != nil {
				t.Fatalf("bad: %#v", obj)
			}
			if got, want := resp.Code, http.StatusUnauthorized; got != want {
				t.Fatalf("got %d want %d", got, want)
			}
			if !strings.Contains(resp.Body.String(), "ACL support disabled") {
				t.Fatalf("bad: %#v", resp)
			}
		})
	}
}

func makeTestACL(t *testing.T, srv *HTTPServer) string {
	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"Name":  "User Token",
		"Type":  "client",
		"Rules": "",
	}
	enc.Encode(raw)

	req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", body)
	resp := httptest.NewRecorder()
	obj, err := srv.ACLCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	aclResp := obj.(aclCreateResponse)
	return aclResp.ID
}

func TestACL_Bootstrap(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_master_token = ""
	`)
	defer a.Shutdown()

	tests := []struct {
		name   string
		method string
		code   int
		token  bool
	}{
		{"bootstrap", "PUT", http.StatusOK, true},
		{"not again", "PUT", http.StatusForbidden, false},
	}
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, "/v1/acl/bootstrap", nil)
			out, err := a.srv.ACLBootstrap(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got, want := resp.Code, tt.code; got != want {
				t.Fatalf("got %d want %d", got, want)
			}
			if tt.token {
				wrap, ok := out.(aclCreateResponse)
				if !ok {
					t.Fatalf("bad: %T", out)
				}
				if len(wrap.ID) != len("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx") {
					t.Fatalf("bad: %v", wrap)
				}
			} else {
				if out != nil {
					t.Fatalf("bad: %T", out)
				}
			}
		})
	}
}

func TestACL_Update(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	id := makeTestACL(t, a.srv)

	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"ID":    id,
		"Name":  "User Token 2",
		"Type":  "client",
		"Rules": "",
	}
	enc.Encode(raw)

	req, _ := http.NewRequest("PUT", "/v1/acl/update?token=root", body)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ACLUpdate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	aclResp := obj.(aclCreateResponse)
	if aclResp.ID != id {
		t.Fatalf("bad: %v", aclResp)
	}
}

func TestACL_UpdateUpsert(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"ID":    "my-old-id",
		"Name":  "User Token 2",
		"Type":  "client",
		"Rules": "",
	}
	enc.Encode(raw)

	req, _ := http.NewRequest("PUT", "/v1/acl/update?token=root", body)
	resp := httptest.NewRecorder()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	obj, err := a.srv.ACLUpdate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	aclResp := obj.(aclCreateResponse)
	if aclResp.ID != "my-old-id" {
		t.Fatalf("bad: %v", aclResp)
	}
}

func TestACL_Destroy(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	id := makeTestACL(t, a.srv)
	req, _ := http.NewRequest("PUT", "/v1/acl/destroy/"+id+"?token=root", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ACLDestroy(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp, ok := obj.(bool); !ok || !resp {
		t.Fatalf("should work")
	}

	req, _ = http.NewRequest("GET", "/v1/acl/info/"+id, nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.ACLGet(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	respObj, ok := obj.(structs.ACLs)
	if !ok {
		t.Fatalf("should work")
	}
	if len(respObj) != 0 {
		t.Fatalf("bad: %v", respObj)
	}
}

func TestACL_Clone(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	id := makeTestACL(t, a.srv)

	req, _ := http.NewRequest("PUT", "/v1/acl/clone/"+id, nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.ACLClone(resp, req)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	req, _ = http.NewRequest("PUT", "/v1/acl/clone/"+id+"?token=root", nil)
	resp = httptest.NewRecorder()
	obj, err := a.srv.ACLClone(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	aclResp, ok := obj.(aclCreateResponse)
	if !ok {
		t.Fatalf("should work: %#v %#v", obj, resp)
	}
	if aclResp.ID == id {
		t.Fatalf("bad id")
	}

	req, _ = http.NewRequest("GET", "/v1/acl/info/"+aclResp.ID, nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.ACLGet(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	respObj, ok := obj.(structs.ACLs)
	if !ok {
		t.Fatalf("should work")
	}
	if len(respObj) != 1 {
		t.Fatalf("bad: %v", respObj)
	}
}

func TestACL_Get(t *testing.T) {
	t.Parallel()
	t.Run("wrong id", func(t *testing.T) {
		a := NewTestAgent(t.Name(), TestACLConfig())
		defer a.Shutdown()

		req, _ := http.NewRequest("GET", "/v1/acl/info/nope", nil)
		resp := httptest.NewRecorder()
		testrpc.WaitForLeader(t, a.RPC, "dc1")
		obj, err := a.srv.ACLGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.ACLs)
		if !ok {
			t.Fatalf("should work")
		}
		if respObj == nil || len(respObj) != 0 {
			t.Fatalf("bad: %v", respObj)
		}
	})

	t.Run("right id", func(t *testing.T) {
		a := NewTestAgent(t.Name(), TestACLConfig())
		defer a.Shutdown()

		testrpc.WaitForLeader(t, a.RPC, "dc1")
		id := makeTestACL(t, a.srv)

		req, _ := http.NewRequest("GET", "/v1/acl/info/"+id, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.ACLs)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 1 {
			t.Fatalf("bad: %v", respObj)
		}
	})
}

func TestACL_List(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
	var ids []string
	for i := 0; i < 10; i++ {
		ids = append(ids, makeTestACL(t, a.srv))
	}

	req, _ := http.NewRequest("GET", "/v1/acl/list?token=root", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ACLList(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	respObj, ok := obj.(structs.ACLs)
	if !ok {
		t.Fatalf("should work")
	}

	// 10 + anonymous + master
	if len(respObj) != 12 {
		t.Fatalf("bad: %v", respObj)
	}
}

func TestACLReplicationStatus(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/acl/replication", nil)
	resp := httptest.NewRecorder()
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	obj, err := a.srv.ACLReplicationStatus(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, ok := obj.(structs.ACLReplicationStatus)
	if !ok {
		t.Fatalf("should work")
	}
}
