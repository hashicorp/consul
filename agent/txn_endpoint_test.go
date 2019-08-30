package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/raft"
	"github.com/pascaldekloe/goe/verify"

	"github.com/hashicorp/consul/agent/structs"
)

func TestTxnEndpoint_Bad_JSON(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	buf := bytes.NewBuffer([]byte("{"))
	req, _ := http.NewRequest("PUT", "/v1/txn", buf)
	resp := httptest.NewRecorder()
	if _, err := a.srv.Txn(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("Failed to parse")) {
		t.Fatalf("expected conflicting args error")
	}
}

func TestTxnEndpoint_Bad_Size_Item(t *testing.T) {
	t.Parallel()
	testIt := func(agent *TestAgent, wantPass bool) {
		value := strings.Repeat("X", 3*raft.SuggestedMaxDataSize)
		value = base64.StdEncoding.EncodeToString([]byte(value))
		buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
 [
     {
         "KV": {
             "Verb": "set",
             "Key": "key",
             "Value": %q
         }
     }
 ]
 `, value)))
		req, _ := http.NewRequest("PUT", "/v1/txn", buf)
		resp := httptest.NewRecorder()
		if _, err := agent.srv.Txn(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 413 && !wantPass {
			t.Fatalf("expected 413, got %d", resp.Code)
		}
		if resp.Code != 200 && wantPass {
			t.Fatalf("expected 200, got %d", resp.Code)
		}
	}

	t.Run("toobig", func(t *testing.T) {
		a := NewTestAgent(t, t.Name(), "")
		testIt(a, false)
		a.Shutdown()
	})

	t.Run("allowed", func(t *testing.T) {
		a := NewTestAgent(t, t.Name(), "limits = { kv_max_value_size = 123456789 }")
		testIt(a, true)
		a.Shutdown()
	})
}

func TestTxnEndpoint_Bad_Size_Net(t *testing.T) {
	t.Parallel()

	testIt := func(agent *TestAgent, wantPass bool) {
		value := strings.Repeat("X", 3*raft.SuggestedMaxDataSize)
		value = base64.StdEncoding.EncodeToString([]byte(value))
		buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
 [
     {
         "KV": {
             "Verb": "set",
             "Key": "key1",
             "Value": %q
         }
     },
     {
         "KV": {
             "Verb": "set",
             "Key": "key1",
             "Value": %q
         }
     },
     {
         "KV": {
             "Verb": "set",
             "Key": "key1",
             "Value": %q
         }
     }
 ]
 `, value, value, value)))
		req, _ := http.NewRequest("PUT", "/v1/txn", buf)
		resp := httptest.NewRecorder()
		if _, err := agent.srv.Txn(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 413 && !wantPass {
			t.Fatalf("expected 413, got %d", resp.Code)
		}
		if resp.Code != 200 && wantPass {
			t.Fatalf("expected 200, got %d", resp.Code)
		}
	}

	t.Run("toobig", func(t *testing.T) {
		a := NewTestAgent(t, t.Name(), "")
		testIt(a, false)
		a.Shutdown()
	})

	t.Run("allowed", func(t *testing.T) {
		a := NewTestAgent(t, t.Name(), "limits = { kv_max_value_size = 123456789 }")
		testIt(a, true)
		a.Shutdown()
	})
}

func TestTxnEndpoint_Bad_Size_Ops(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
 [
     %s
     {
         "KV": {
             "Verb": "set",
             "Key": "key",
             "Value": ""
         }
     }
 ]
 `, strings.Repeat(`{ "KV": { "Verb": "get", "Key": "key" } },`, 2*maxTxnOps))))
	req, _ := http.NewRequest("PUT", "/v1/txn", buf)
	resp := httptest.NewRecorder()
	if _, err := a.srv.Txn(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 413 {
		t.Fatalf("expected 413, got %d", resp.Code)
	}
}

func TestTxnEndpoint_KV_Actions(t *testing.T) {
	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, t.Name(), "")
		defer a.Shutdown()
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")

		// Make sure all incoming fields get converted properly to the internal
		// RPC format.
		var index uint64
		id := makeTestSession(t, a.srv)
		{
			buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
 [
     {
         "KV": {
             "Verb": "lock",
             "Key": "key",
             "Value": "aGVsbG8gd29ybGQ=",
             "Flags": 23,
             "Session": %q
         }
     },
     {
         "KV": {
             "Verb": "get",
             "Key": "key"
         }
     }
 ]
 `, id)))
			req, _ := http.NewRequest("PUT", "/v1/txn", buf)
			resp := httptest.NewRecorder()
			obj, err := a.srv.Txn(resp, req)
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
			index = txnResp.Results[0].KV.ModifyIndex
			expected := structs.TxnResponse{
				Results: structs.TxnResults{
					&structs.TxnResult{
						KV: &structs.DirEntry{
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
					&structs.TxnResult{
						KV: &structs.DirEntry{
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
			}
			if !reflect.DeepEqual(txnResp, expected) {
				t.Fatalf("bad: %v", txnResp)
			}
		}

		// Do a read-only transaction that should get routed to the
		// fast-path endpoint.
		{
			buf := bytes.NewBuffer([]byte(`
 [
     {
         "KV": {
             "Verb": "get",
             "Key": "key"
         }
     },
     {
         "KV": {
             "Verb": "get-tree",
             "Key": "key"
         }
     }
 ]
 `))
			req, _ := http.NewRequest("PUT", "/v1/txn", buf)
			resp := httptest.NewRecorder()
			obj, err := a.srv.Txn(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if resp.Code != 200 {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			header := resp.Header().Get("X-Consul-KnownLeader")
			if header != "true" {
				t.Fatalf("bad: %v", header)
			}
			header = resp.Header().Get("X-Consul-LastContact")
			if header != "0" {
				t.Fatalf("bad: %v", header)
			}

			txnResp, ok := obj.(structs.TxnReadResponse)
			if !ok {
				t.Fatalf("bad type: %T", obj)
			}
			expected := structs.TxnReadResponse{
				TxnResponse: structs.TxnResponse{
					Results: structs.TxnResults{
						&structs.TxnResult{
							KV: &structs.DirEntry{
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
						&structs.TxnResult{
							KV: &structs.DirEntry{
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
				QueryMeta: structs.QueryMeta{
					KnownLeader: true,
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
         "KV": {
             "Verb": "cas",
             "Key": "key",
             "Value": "Z29vZGJ5ZSB3b3JsZA==",
             "Index": %d
         }
     },
     {
         "KV": {
             "Verb": "get",
             "Key": "key"
         }
     }
 ]
 `, index)))
			req, _ := http.NewRequest("PUT", "/v1/txn", buf)
			resp := httptest.NewRecorder()
			obj, err := a.srv.Txn(resp, req)
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
			modIndex := txnResp.Results[0].KV.ModifyIndex
			expected := structs.TxnResponse{
				Results: structs.TxnResults{
					&structs.TxnResult{
						KV: &structs.DirEntry{
							Key:     "key",
							Value:   nil,
							Session: id,
							RaftIndex: structs.RaftIndex{
								CreateIndex: index,
								ModifyIndex: modIndex,
							},
						},
					},
					&structs.TxnResult{
						KV: &structs.DirEntry{
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
			}
			if !reflect.DeepEqual(txnResp, expected) {
				t.Fatalf("bad: %v", txnResp)
			}
		}
	})

	// Verify an error inside a transaction.
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, t.Name(), "")
		defer a.Shutdown()

		buf := bytes.NewBuffer([]byte(`
 [
     {
         "KV": {
             "Verb": "lock",
             "Key": "key",
             "Value": "aGVsbG8gd29ybGQ=",
             "Session": "nope"
         }
     },
     {
         "KV": {
             "Verb": "get",
             "Key": "key"
         }
     }
 ]
 `))
		req, _ := http.NewRequest("PUT", "/v1/txn", buf)
		resp := httptest.NewRecorder()
		if _, err := a.srv.Txn(resp, req); err != nil {
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

func TestTxnEndpoint_UpdateCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Make sure the fields of a check are handled correctly when both creating and
	// updating, and test both sets of duration fields to ensure backwards compatibility.
	buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
[
	{
		"Check": {
			"Verb": "set",
			"Check": {
				"Node": "%s",
				"CheckID": "nodecheck",
				"Name": "Node http check",
				"Status": "critical",
				"Notes": "Http based health check",
				"Output": "",
				"ServiceID": "",
				"ServiceName": "",
				"Definition": {
					"Interval": "6s",
					"Timeout": "6s",
					"DeregisterCriticalServiceAfter": "6s",
					"HTTP": "http://localhost:8000",
					"TLSSkipVerify": true
				}
			}
		}
	},
	{
		"Check": {
			"Verb": "set",
			"Check": {
				"Node": "%s",
				"CheckID": "nodecheck",
				"Name": "Node http check",
				"Status": "passing",
				"Notes": "Http based health check",
				"Output": "success",
				"ServiceID": "",
				"ServiceName": "",
				"Definition": {
					"Interval": "10s",
					"Timeout": "10s",
					"DeregisterCriticalServiceAfter": "15m",
					"HTTP": "http://localhost:9000",
					"TLSSkipVerify": false
				}
			}
		}
	},
	{
		"Check": {
			"Verb": "set",
			"Check": {
				"Node": "%s",
				"CheckID": "nodecheck",
				"Name": "Node http check",
				"Status": "passing",
				"Notes": "Http based health check",
				"Output": "success",
				"ServiceID": "",
				"ServiceName": "",
				"Definition": {
					"IntervalDuration": "15s",
					"TimeoutDuration": "15s",
					"DeregisterCriticalServiceAfterDuration": "30m",
					"HTTP": "http://localhost:9000",
					"TLSSkipVerify": false
				}
			}
		}
	}
]
`, a.config.NodeName, a.config.NodeName, a.config.NodeName)))
	req, _ := http.NewRequest("PUT", "/v1/txn", buf)
	resp := httptest.NewRecorder()
	obj, err := a.srv.Txn(resp, req)
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
	if len(txnResp.Results) != 3 {
		t.Fatalf("bad: %v", txnResp)
	}
	index := txnResp.Results[0].Check.ModifyIndex
	expected := structs.TxnResponse{
		Results: structs.TxnResults{
			&structs.TxnResult{
				Check: &structs.HealthCheck{
					Node:    a.config.NodeName,
					CheckID: "nodecheck",
					Name:    "Node http check",
					Status:  api.HealthCritical,
					Notes:   "Http based health check",
					Definition: structs.HealthCheckDefinition{
						Interval:                       6 * time.Second,
						Timeout:                        6 * time.Second,
						DeregisterCriticalServiceAfter: 6 * time.Second,
						HTTP:                           "http://localhost:8000",
						TLSSkipVerify:                  true,
					},
					RaftIndex: structs.RaftIndex{
						CreateIndex: index,
						ModifyIndex: index,
					},
				},
			},
			&structs.TxnResult{
				Check: &structs.HealthCheck{
					Node:    a.config.NodeName,
					CheckID: "nodecheck",
					Name:    "Node http check",
					Status:  api.HealthPassing,
					Notes:   "Http based health check",
					Output:  "success",
					Definition: structs.HealthCheckDefinition{
						Interval:                       10 * time.Second,
						Timeout:                        10 * time.Second,
						DeregisterCriticalServiceAfter: 15 * time.Minute,
						HTTP:                           "http://localhost:9000",
						TLSSkipVerify:                  false,
					},
					RaftIndex: structs.RaftIndex{
						CreateIndex: index,
						ModifyIndex: index,
					},
				},
			},
			&structs.TxnResult{
				Check: &structs.HealthCheck{
					Node:    a.config.NodeName,
					CheckID: "nodecheck",
					Name:    "Node http check",
					Status:  api.HealthPassing,
					Notes:   "Http based health check",
					Output:  "success",
					Definition: structs.HealthCheckDefinition{
						Interval:                       15 * time.Second,
						Timeout:                        15 * time.Second,
						DeregisterCriticalServiceAfter: 30 * time.Minute,
						HTTP:                           "http://localhost:9000",
						TLSSkipVerify:                  false,
					},
					RaftIndex: structs.RaftIndex{
						CreateIndex: index,
						ModifyIndex: index,
					},
				},
			},
		},
	}
	verify.Values(t, "", txnResp, expected)
}

func TestConvertOps_ContentLength(t *testing.T) {
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	jsonBody := `[
     {
         "KV": {
             "Verb": "set",
             "Key": "key1",
             "Value": "aGVsbG8gd29ybGQ="
         }
     }
 ]`

	tests := []struct {
		contentLength string
		ok            bool
	}{
		{"", true},
		{strconv.Itoa(len(jsonBody)), true},
		{strconv.Itoa(raft.SuggestedMaxDataSize), true},
		{strconv.Itoa(raft.SuggestedMaxDataSize + 100), false},
	}

	for _, tc := range tests {
		t.Run("contentLength: "+tc.contentLength, func(t *testing.T) {
			resp := httptest.NewRecorder()
			var body bytes.Buffer

			// Doesn't matter what the request body size actually is, as we only
			// check 'Content-Length' header in this test anyway.
			body.WriteString(jsonBody)

			req := httptest.NewRequest("POST", "http://foo.com", &body)
			req.Header.Add("Content-Length", tc.contentLength)

			_, _, ok := a.srv.convertOps(resp, req)
			if ok != tc.ok {
				t.Fatal("ok != tc.ok")
			}

		})

	}

}
