package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

func TestTxnEndpoint_Bad_JSON(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		buf := bytes.NewBuffer([]byte("{"))
		req, err := http.NewRequest("PUT", "/v1/txn", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		if _, err := srv.Txn(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
		if !bytes.Contains(resp.Body.Bytes(), []byte("Failed to parse")) {
			t.Fatalf("expected conflicting args error")
		}
	})
}

func TestTxnEndpoint_Bad_Method(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		buf := bytes.NewBuffer([]byte("{}"))
		req, err := http.NewRequest("GET", "/v1/txn", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		if _, err := srv.Txn(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 405 {
			t.Fatalf("expected 405, got %d", resp.Code)
		}
	})
}

func TestTxnEndpoint_KVS_Actions(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		// Make sure all incoming fields get converted properly to the internal
		// RPC format.
		var index uint64
		id := makeTestSession(t, srv)
		{
			buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
[
    {
        "Op": "lock",
        "Key": "key",
        "Value": "aGVsbG8gd29ybGQ=",
        "Flags": 23,
        "Session": %q
    },
    {
        "Op": "get",
        "Key": "key"
    }
]
`, id)))
			req, err := http.NewRequest("PUT", "/v1/txn", buf)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			resp := httptest.NewRecorder()
			obj, err := srv.Txn(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if resp.Code != 200 {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			atomic, ok := obj.(structs.KVSAtomicResponse)
			if !ok {
				t.Fatalf("bad type: %T", obj)
			}
			if len(atomic.Results) != 2 {
				t.Fatalf("bad: %v", atomic)
			}
			index = atomic.Results[0].ModifyIndex
			expected := structs.KVSAtomicResponse{
				Results: structs.DirEntries{
					&structs.DirEntry{
						Key:       "key",
						Value:     nil,
						Flags:     23,
						Session:   id,
						LockIndex: 1,
						RaftIndex: structs.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
					&structs.DirEntry{
						Key:       "key",
						Value:     []byte("hello world"),
						Flags:     23,
						Session:   id,
						LockIndex: 1,
						RaftIndex: structs.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
				},
			}
			if !reflect.DeepEqual(atomic, expected) {
				t.Fatalf("bad: %v", atomic)
			}
		}

		// Now that we have an index we can do a CAS to make sure the
		// index field gets translated to the RPC format.
		{
			buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
[
    {
        "Op": "cas",
        "Key": "key",
        "Value": "Z29vZGJ5ZSB3b3JsZA==",
        "Index": %d
    },
    {
        "Op": "get",
        "Key": "key"
    }
]
`, index)))
			req, err := http.NewRequest("PUT", "/v1/txn", buf)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			resp := httptest.NewRecorder()
			obj, err := srv.Txn(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if resp.Code != 200 {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			atomic, ok := obj.(structs.KVSAtomicResponse)
			if !ok {
				t.Fatalf("bad type: %T", obj)
			}
			if len(atomic.Results) != 2 {
				t.Fatalf("bad: %v", atomic)
			}
			modIndex := atomic.Results[0].ModifyIndex
			expected := structs.KVSAtomicResponse{
				Results: structs.DirEntries{
					&structs.DirEntry{
						Key:     "key",
						Value:   nil,
						Session: id,
						RaftIndex: structs.RaftIndex{
							CreateIndex: index,
							ModifyIndex: modIndex,
						},
					},
					&structs.DirEntry{
						Key:     "key",
						Value:   []byte("goodbye world"),
						Session: id,
						RaftIndex: structs.RaftIndex{
							CreateIndex: index,
							ModifyIndex: modIndex,
						},
					},
				},
			}
			if !reflect.DeepEqual(atomic, expected) {
				t.Fatalf("bad: %v", atomic)
			}
		}
	})

	// Verify an error inside a transaction.
	httpTest(t, func(srv *HTTPServer) {
		buf := bytes.NewBuffer([]byte(`
[
    {
        "Op": "lock",
        "Key": "key",
        "Value": "aGVsbG8gd29ybGQ=",
        "Session": "nope"
    },
    {
        "Op": "get",
        "Key": "key"
    }
]
`))
		req, err := http.NewRequest("PUT", "/v1/txn", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		if _, err = srv.Txn(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 409 {
			t.Fatalf("expected 409, got %d", resp.Code)
		}
		if !bytes.Contains(resp.Body.Bytes(), []byte("failed session lookup")) {
			t.Fatalf("bad: %s", resp.Body.String())
		}
	})
}
