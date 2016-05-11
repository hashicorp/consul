package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

func TestTxnEndpoint_Bad_Size(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
[
    {
        "KVS": {
            "Verb": "set",
            "Key": "key",
            "Value": %q
        }
    }
]
`, strings.Repeat("bad", 2*maxKVSize))))
		req, err := http.NewRequest("PUT", "/v1/txn", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		if _, err := srv.Txn(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 413 {
			t.Fatalf("expected 413, got %d", resp.Code)
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
        "KVS": {
            "Verb": "lock",
            "Key": "key",
            "Value": "aGVsbG8gd29ybGQ=",
            "Flags": 23,
            "Session": %q
        }
    },
    {
        "KVS": {
            "Verb": "get",
            "Key": "key"
        }
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

			txnResp, ok := obj.(structs.TxnResponse)
			if !ok {
				t.Fatalf("bad type: %T", obj)
			}
			if len(txnResp.Results) != 2 {
				t.Fatalf("bad: %v", txnResp)
			}
			index = txnResp.Results[0].KVS.DirEnt.ModifyIndex
			expected := structs.TxnResponse{
				Results: structs.TxnResults{
					&structs.TxnResult{
						KVS: &structs.TxnKVSResult{
							DirEnt: &structs.DirEntry{
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
						},
					},
					&structs.TxnResult{
						KVS: &structs.TxnKVSResult{
							DirEnt: &structs.DirEntry{
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
					},
				},
			}
			if !reflect.DeepEqual(txnResp, expected) {
				t.Fatalf("bad: %v", txnResp)
			}
		}

		// Now that we have an index we can do a CAS to make sure the
		// index field gets translated to the RPC format.
		{
			buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
[
    {
        "KVS": {
            "Verb": "cas",
            "Key": "key",
            "Value": "Z29vZGJ5ZSB3b3JsZA==",
            "Index": %d
        }
    },
    {
        "KVS": {
            "Verb": "get",
            "Key": "key"
        }
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

			txnResp, ok := obj.(structs.TxnResponse)
			if !ok {
				t.Fatalf("bad type: %T", obj)
			}
			if len(txnResp.Results) != 2 {
				t.Fatalf("bad: %v", txnResp)
			}
			modIndex := txnResp.Results[0].KVS.DirEnt.ModifyIndex
			expected := structs.TxnResponse{
				Results: structs.TxnResults{
					&structs.TxnResult{
						KVS: &structs.TxnKVSResult{
							DirEnt: &structs.DirEntry{
								Key:     "key",
								Value:   nil,
								Session: id,
								RaftIndex: structs.RaftIndex{
									CreateIndex: index,
									ModifyIndex: modIndex,
								},
							},
						},
					},
					&structs.TxnResult{
						KVS: &structs.TxnKVSResult{
							DirEnt: &structs.DirEntry{
								Key:     "key",
								Value:   []byte("goodbye world"),
								Session: id,
								RaftIndex: structs.RaftIndex{
									CreateIndex: index,
									ModifyIndex: modIndex,
								},
							},
						},
					},
				},
			}
			if !reflect.DeepEqual(txnResp, expected) {
				t.Fatalf("bad: %v", txnResp)
			}
		}
	})

	// Verify an error inside a transaction.
	httpTest(t, func(srv *HTTPServer) {
		buf := bytes.NewBuffer([]byte(`
[
    {
        "KVS": {
            "Verb": "lock",
            "Key": "key",
            "Value": "aGVsbG8gd29ybGQ=",
            "Session": "nope"
        }
    },
    {
        "KVS": {
            "Verb": "get",
            "Key": "key"
        }
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
