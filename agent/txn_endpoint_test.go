// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestTxnEndpoint_Bad_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	buf := bytes.NewBuffer([]byte("{"))
	req, _ := http.NewRequest("PUT", "/v1/txn", buf)
	resp := httptest.NewRecorder()
	_, err := a.srv.Txn(resp, req)
	require.True(t, isHTTPBadRequest(err), fmt.Sprintf("Expected bad request HTTP error but got %v", err))
	if !strings.Contains(err.Error(), "Failed to parse") {
		t.Fatalf("expected conflicting args error")
	}
}

func TestTxnEndpoint_Bad_Size_Item(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testIt := func(t *testing.T, agent *TestAgent, wantPass bool) {
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
		_, err := agent.srv.Txn(resp, req)

		if wantPass {
			if err != nil {
				t.Fatalf("err: %v", err)
			}
		} else {
			if httpErr, ok := err.(HTTPError); ok {
				if httpErr.StatusCode != 413 {
					t.Fatalf("expected 413 but got %d", httpErr.StatusCode)
				}
			} else {
				t.Fatalf("excected HTTP error but got %v", err)
			}
		}
	}

	t.Run("exceeds default limits", func(t *testing.T) {
		a := NewTestAgent(t, "")
		testIt(t, a, false)
		a.Shutdown()
	})

	t.Run("exceeds configured max txn len", func(t *testing.T) {
		a := NewTestAgent(t, "limits = { txn_max_req_len = 700000 }")
		testIt(t, a, false)
		a.Shutdown()
	})

	t.Run("exceeds default max kv value size", func(t *testing.T) {
		a := NewTestAgent(t, "limits = { txn_max_req_len = 123456789 }")
		testIt(t, a, false)
		a.Shutdown()
	})

	t.Run("allowed", func(t *testing.T) {
		a := NewTestAgent(t, `
limits = {
	txn_max_req_len = 123456789
	kv_max_value_size = 123456789
}`)
		testIt(t, a, true)
		a.Shutdown()
	})
}

func TestTxnEndpoint_Bad_Size_Net(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		_, err := agent.srv.Txn(resp, req)

		if wantPass {
			if err != nil {
				t.Fatalf("err: %v", err)
			}
		} else {
			if httpErr, ok := err.(HTTPError); ok {
				if httpErr.StatusCode != 413 {
					t.Fatalf("expected 413 but got %d", httpErr.StatusCode)
				}
			} else {
				t.Fatalf("excected HTTP error but got %v", err)
			}
		}
	}

	t.Run("exceeds default limits", func(t *testing.T) {
		a := NewTestAgent(t, "")
		testIt(a, false)
		a.Shutdown()
	})

	t.Run("exceeds configured max txn len", func(t *testing.T) {
		a := NewTestAgent(t, "limits = { txn_max_req_len = 700000 }")
		testIt(a, false)
		a.Shutdown()
	})

	t.Run("exceeds default max kv value size", func(t *testing.T) {
		a := NewTestAgent(t, "limits = { txn_max_req_len = 123456789 }")
		testIt(a, false)
		a.Shutdown()
	})

	t.Run("allowed", func(t *testing.T) {
		a := NewTestAgent(t, `
limits = {
	txn_max_req_len = 123456789
	kv_max_value_size = 123456789
}`)
		testIt(a, true)
		a.Shutdown()
	})

	t.Run("allowed kv max backward compatible", func(t *testing.T) {
		a := NewTestAgent(t, "limits = { kv_max_value_size = 123456789 }")
		testIt(a, true)
		a.Shutdown()
	})
}

func TestTxnEndpoint_Bad_Size_Ops(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	_, err := a.srv.Txn(resp, req)

	if httpErr, ok := err.(HTTPError); ok {
		if httpErr.StatusCode != 413 {
			t.Fatalf("expected 413 but got %d", httpErr.StatusCode)
		}
	} else {
		t.Fatalf("expected HTTP error but got %v", err)
	}
}

func TestTxnEndpoint_KV_Actions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
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
			entMeta := txnResp.Results[0].KV.EnterpriseMeta

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
							EnterpriseMeta: entMeta,
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
							EnterpriseMeta: entMeta,
						},
					},
				},
			}
			assert.Equal(t, expected, txnResp)
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
			entMeta := txnResp.Results[0].KV.EnterpriseMeta
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
								EnterpriseMeta: entMeta,
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
								EnterpriseMeta: entMeta,
							},
						},
					},
				},
				QueryMeta: structs.QueryMeta{
					KnownLeader: true,
					Index:       1,
				},
			}
			assert.Equal(t, expected, txnResp)
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
			entMeta := txnResp.Results[0].KV.EnterpriseMeta

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
							EnterpriseMeta: entMeta,
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
							EnterpriseMeta: entMeta,
						},
					},
				},
			}
			assert.Equal(t, expected, txnResp)
		}
	})

	// Verify an error inside a transaction.
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
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
		if !bytes.Contains(resp.Body.Bytes(), []byte("invalid session")) {
			t.Fatalf("bad: %s", resp.Body.String())
		}
	})
}

func TestTxnEndpoint_UpdateCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
				"ExposedPort": 5678,
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
	require.NoError(t, err)
	require.Equal(t, 200, resp.Code, resp.Body)

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
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
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
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			&structs.TxnResult{
				Check: &structs.HealthCheck{
					Node:        a.config.NodeName,
					CheckID:     "nodecheck",
					Name:        "Node http check",
					Status:      api.HealthPassing,
					Notes:       "Http based health check",
					Output:      "success",
					ExposedPort: 5678,
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
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
	}
	assert.Equal(t, expected, txnResp)
}

func TestTxnEndpoint_NodeService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Make sure the fields of a check are handled correctly when both creating and
	// updating, and test both sets of duration fields to ensure backwards compatibility.
	buf := bytes.NewBuffer([]byte(fmt.Sprintf(`
[
	{
		"Service": {
		  	"Verb": "set",
		  	"Node": "%s",
		  	"Service": {
				"Service": "test",
				"Port": 4444
		  	}
		}
	},
	{
		"Service": {
			"Verb": "set",
			"Node": "%s",
			"Service": {
				"Service": "test-sidecar-proxy",
				"Port": 20000,
				"Kind": "connect-proxy",
				"Proxy": {
				  	"DestinationServiceName": "test",
				  	"DestinationServiceID": "test",
				  	"LocalServiceAddress": "127.0.0.1",
				  	"LocalServicePort": 4444,
				  	"upstreams": [
						{
							"DestinationName": "fake-backend",
							"LocalBindPort": 25001
						}
				  	]
				}
			}
		}
	}
]
`, a.config.NodeName, a.config.NodeName)))
	req, _ := http.NewRequest("PUT", "/v1/txn", buf)
	resp := httptest.NewRecorder()
	obj, err := a.srv.Txn(resp, req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.Code)

	txnResp, ok := obj.(structs.TxnResponse)
	if !ok {
		t.Fatalf("bad type: %T", obj)
	}
	require.Equal(t, 2, len(txnResp.Results))

	index := txnResp.Results[0].Service.ModifyIndex
	expected := structs.TxnResponse{
		Results: structs.TxnResults{
			&structs.TxnResult{
				Service: &structs.NodeService{
					Service: "test",
					ID:      "test",
					Port:    4444,
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
					RaftIndex: structs.RaftIndex{
						CreateIndex: index,
						ModifyIndex: index,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			&structs.TxnResult{
				Service: &structs.NodeService{
					Service: "test-sidecar-proxy",
					ID:      "test-sidecar-proxy",
					Port:    20000,
					Kind:    "connect-proxy",
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "test",
						DestinationServiceID:   "test",
						LocalServiceAddress:    "127.0.0.1",
						LocalServicePort:       4444,
					},
					TaggedAddresses: map[string]structs.ServiceAddress{
						"consul-virtual": {
							Address: "240.0.0.1",
							Port:    20000,
						},
					},
					RaftIndex: structs.RaftIndex{
						CreateIndex: index,
						ModifyIndex: index,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
	}
	assert.Equal(t, expected, txnResp)
}

func TestTxnEndpoint_OperationsSize(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("too-many-operations", func(t *testing.T) {
		var ops []api.TxnOp
		agent := NewTestAgent(t, "limits = { txn_max_req_len = 700000 }")

		for i := 0; i < 130; i++ {
			ops = append(ops, api.TxnOp{
				KV: &api.KVTxnOp{
					Verb:  api.KVSet,
					Key:   "key",
					Value: []byte("test"),
				},
			})
		}

		req, _ := http.NewRequest("PUT", "/v1/txn", jsonBody(ops))
		resp := httptest.NewRecorder()
		raw, err := agent.srv.Txn(resp, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Transaction contains too many operations")
		require.Nil(t, raw)
		agent.Shutdown()
	})

	t.Run("allowed", func(t *testing.T) {
		var ops []api.TxnOp
		agent := NewTestAgent(t, "limits = { txn_max_req_len = 700000 }")

		for i := 0; i < 128; i++ {
			ops = append(ops, api.TxnOp{
				KV: &api.KVTxnOp{
					Verb:  api.KVSet,
					Key:   "key",
					Value: []byte("test"),
				},
			})
		}

		req, _ := http.NewRequest("PUT", "/v1/txn", jsonBody(ops))
		resp := httptest.NewRecorder()
		raw, err := agent.srv.Txn(resp, req)
		require.NoError(t, err)
		require.NotNil(t, raw)
		agent.Shutdown()
	})
}
