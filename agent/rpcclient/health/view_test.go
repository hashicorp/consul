package health

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

/*

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/types"
)

func TestStreamingHealthServices_EmptySnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	namespace := pbcommon.DefaultEnterpriseMeta.Namespace
	client := NewTestStreamingClient(namespace)
	typ := StreamingHealthServices{deps: MaterializerDeps{
		Client: client,
		Logger: hclog.Default(),
	}}

	// Initially there are no services registered. Server should send an
	// EndOfSnapshot message immediately with index of 1.
	client.QueueEvents(newEndOfSnapshotEvent(1))

	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:     "dc1",
		ServiceName:    "web",
		EnterpriseMeta: structs.NewEnterpriseMeta(namespace),
	}
	empty := &structs.IndexedCheckServiceNodes{
		Nodes: structs.CheckServiceNodes{},
		QueryMeta: structs.QueryMeta{
			Index: 1,
		},
	}

	runStep(t, "empty snapshot returned", func(t *testing.T) {
		// Fetch should return an empty
		// result of the right type with a non-zero index, and in the background begin
		// streaming updates.
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(1), result.Index)
		require.Equal(t, empty, result.Value)

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "blocks for timeout", func(t *testing.T) {
		// Subsequent fetch should block for the timeout
		start := time.Now()
		opts.Timeout = 200 * time.Millisecond
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until timeout")

		require.Equal(t, opts.MinIndex, result.Index, "result index should not have changed")
		require.Equal(t, empty, result.Value, "result value should not have changed")

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "blocks until update", func(t *testing.T) {
		// Make another blocking query with a longer timeout and trigger an update
		// event part way through.
		start := time.Now()
		go func() {
			time.Sleep(200 * time.Millisecond)
			client.QueueEvents(newEventServiceHealthRegister(4, 1, "web"))
		}()

		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until the event was delivered")
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(4), result.Index, "result index should not have changed")
		require.Len(t, result.Value.(*structs.IndexedCheckServiceNodes).Nodes, 1,
			"result value should contain the new registration")

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "reconnects and resumes after temporary error", func(t *testing.T) {
		client.QueueErr(tempError("broken pipe"))

		// Next fetch will continue to block until timeout and receive the same
		// result.
		start := time.Now()
		opts.Timeout = 200 * time.Millisecond
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until timeout")

		require.Equal(t, opts.MinIndex, result.Index, "result index should not have changed")
		require.Equal(t, opts.LastResult.Value, result.Value, "result value should not have changed")

		opts.MinIndex = result.Index
		opts.LastResult = &result

		// But an update should still be noticed due to reconnection
		client.QueueEvents(newEventServiceHealthRegister(10, 2, "web"))

		start = time.Now()
		opts.Timeout = time.Second
		result, err = typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed = time.Since(start)
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(10), result.Index, "result index should not have changed")
		require.Len(t, result.Value.(*structs.IndexedCheckServiceNodes).Nodes, 2,
			"result value should contain the new registration")

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "returns non-temporary error to watchers", func(t *testing.T) {
		// Wait and send the error while fetcher is waiting
		go func() {
			time.Sleep(200 * time.Millisecond)
			client.QueueErr(errors.New("invalid request"))
		}()

		// Next fetch should return the error
		start := time.Now()
		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.Error(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until error was sent")
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, opts.MinIndex, result.Index, "result index should not have changed")
		// We don't require instances to be returned in same order so we use
		// elementsMatch which is recursive.
		requireResultsSame(t,
			opts.LastResult.Value.(*structs.IndexedCheckServiceNodes),
			result.Value.(*structs.IndexedCheckServiceNodes),
		)

		opts.MinIndex = result.Index
		opts.LastResult = &result

		// But an update should still be noticed due to reconnection
		client.QueueEvents(newEventServiceHealthRegister(opts.MinIndex+5, 3, "web"))

		opts.Timeout = time.Second
		result, err = typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed = time.Since(start)
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, opts.MinIndex+5, result.Index, "result index should not have changed")
		require.Len(t, result.Value.(*structs.IndexedCheckServiceNodes).Nodes, 3,
			"result value should contain the new registration")

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})
}

type tempError string

func (e tempError) Error() string {
	return string(e)
}

func (e tempError) Temporary() bool {
	return true
}

// requireResultsSame compares two IndexedCheckServiceNodes without requiring
// the same order of results (which vary due to map usage internally).
func requireResultsSame(t *testing.T, want, got *structs.IndexedCheckServiceNodes) {
	require.Equal(t, want.Index, got.Index)

	svcIDs := func(csns structs.CheckServiceNodes) []string {
		res := make([]string, 0, len(csns))
		for _, csn := range csns {
			res = append(res, fmt.Sprintf("%s/%s", csn.Node.Node, csn.Service.ID))
		}
		return res
	}

	gotIDs := svcIDs(got.Nodes)
	wantIDs := svcIDs(want.Nodes)

	require.ElementsMatch(t, wantIDs, gotIDs)
}

// getNamespace returns a namespace if namespace support exists, otherwise
// returns the empty string. It allows the same tests to work in both oss and ent
// without duplicating the tests.
func getNamespace(ns string) string {
	meta := structs.NewEnterpriseMeta(ns)
	return meta.NamespaceOrEmpty()
}

func TestOrderingConsistentWithMemDb(t *testing.T) {
	index := uint64(42)
	buildTestNode := func(nodeName string, serviceID string) structs.CheckServiceNode {
		newID, err := uuid.GenerateUUID()
		require.NoError(t, err)
		return structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         types.NodeID(strings.ToUpper(newID)),
				Node:       nodeName,
				Address:    nodeName,
				Datacenter: "dc1",
				RaftIndex: structs.RaftIndex{
					CreateIndex: index,
					ModifyIndex: index,
				},
			},
			Service: &structs.NodeService{
				ID:      serviceID,
				Service: "testService",
				Port:    8080,
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				RaftIndex: structs.RaftIndex{
					CreateIndex: index,
					ModifyIndex: index,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			Checks: []*structs.HealthCheck{},
		}
	}
	zero := buildTestNode("a-zero-node", "testService:1")
	one := buildTestNode("node1", "testService:1")
	two := buildTestNode("node1", "testService:2")
	three := buildTestNode("node2", "testService")
	result := structs.IndexedCheckServiceNodes{
		Nodes: structs.CheckServiceNodes{
			three, two, zero, one,
		},
		QueryMeta: structs.QueryMeta{
			Index: index,
		},
	}
	sortCheckServiceNodes(&result)
	expected := structs.CheckServiceNodes{zero, one, two, three}
	require.Equal(t, expected, result.Nodes)
}

func TestStreamingHealthServices_FullSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	namespace := getNamespace("ns2")
	client := NewTestStreamingClient(namespace)
	typ := StreamingHealthServices{deps: MaterializerDeps{
		Client: client,
		Logger: hclog.Default(),
	}}

	// Create an initial snapshot of 3 instances on different nodes
	registerServiceWeb := func(index uint64, nodeNum int) *pbsubscribe.Event {
		return newEventServiceHealthRegister(index, nodeNum, "web")
	}
	client.QueueEvents(
		registerServiceWeb(5, 1),
		registerServiceWeb(5, 2),
		registerServiceWeb(5, 3),
		newEndOfSnapshotEvent(5))

	// This contains the view state so important we share it between calls.
	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:     "dc1",
		ServiceName:    "web",
		EnterpriseMeta: structs.NewEnterpriseMeta(namespace),
	}

	gatherNodes := func(res interface{}) []string {
		nodes := make([]string, 0, 3)
		r := res.(*structs.IndexedCheckServiceNodes)
		for _, csn := range r.Nodes {
			nodes = append(nodes, csn.Node.Node)
		}
		// Result will be sorted alphabetically the same way as memdb
		return nodes
	}

	runStep(t, "full snapshot returned", func(t *testing.T) {
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		require.ElementsMatch(t, []string{"node1", "node2", "node3"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "blocks until deregistration", func(t *testing.T) {
		// Make another blocking query with a longer timeout and trigger an update
		// event part way through.
		start := time.Now()
		go func() {
			time.Sleep(200 * time.Millisecond)

			// Deregister instance on node1
			client.QueueEvents(newEventServiceHealthDeregister(20, 1, "web"))
		}()

		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until the event was delivered")
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(20), result.Index)
		require.Equal(t, []string{"node2", "node3"}, gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "server reload is respected", func(t *testing.T) {
		// Simulates the server noticing the request's ACL token privs changing. To
		// detect this we'll queue up the new snapshot as a different set of nodes
		// to the first.
		client.QueueErr(status.Error(codes.Aborted, "reset by server"))

		client.QueueEvents(
			registerServiceWeb(50, 3), // overlap existing node
			registerServiceWeb(50, 4),
			registerServiceWeb(50, 5),
			newEndOfSnapshotEvent(50))

		// Make another blocking query with THE SAME index. It should immediately
		// return the new snapshot.
		start := time.Now()
		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(50), result.Index)
		require.Equal(t, []string{"node3", "node4", "node5"}, gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "reconnects and receives new snapshot when server state has changed", func(t *testing.T) {
		client.QueueErr(tempError("temporary connection error"))

		client.QueueEvents(
			newNewSnapshotToFollowEvent(),
			registerServiceWeb(50, 3), // overlap existing node
			registerServiceWeb(50, 4),
			registerServiceWeb(50, 5),
			newEndOfSnapshotEvent(50))

		start := time.Now()
		opts.MinIndex = 49
		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(50), result.Index)
		require.Equal(t, []string{"node3", "node4", "node5"}, gatherNodes(result.Value))
	})
}

func TestStreamingHealthServices_EventBatches(t *testing.T) {
	namespace := getNamespace("ns3")
	client := NewTestStreamingClient(namespace)
	typ := StreamingHealthServices{deps: MaterializerDeps{
		Client: client,
		Logger: hclog.Default(),
	}}

	// Create an initial snapshot of 3 instances but in a single event batch
	batchEv := newEventBatchWithEvents(
		newEventServiceHealthRegister(5, 1, "web"),
		newEventServiceHealthRegister(5, 2, "web"),
		newEventServiceHealthRegister(5, 3, "web"))
	client.QueueEvents(
		batchEv,
		newEndOfSnapshotEvent(5))

	// This contains the view state so important we share it between calls.
	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:     "dc1",
		ServiceName:    "web",
		EnterpriseMeta: structs.NewEnterpriseMeta(namespace),
	}

	gatherNodes := func(res interface{}) []string {
		nodes := make([]string, 0, 3)
		r := res.(*structs.IndexedCheckServiceNodes)
		for _, csn := range r.Nodes {
			nodes = append(nodes, csn.Node.Node)
		}
		return nodes
	}

	runStep(t, "full snapshot returned", func(t *testing.T) {
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		require.ElementsMatch(t, []string{"node1", "node2", "node3"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "batched updates work too", func(t *testing.T) {
		// Simulate multiple registrations happening in one Txn (so all have same
		// index)
		batchEv := newEventBatchWithEvents(
			// Deregister an existing node
			newEventServiceHealthDeregister(20, 1, "web"),
			// Register another
			newEventServiceHealthRegister(20, 4, "web"),
		)
		client.QueueEvents(batchEv)
		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(20), result.Index)
		require.ElementsMatch(t, []string{"node2", "node3", "node4"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})
}

func TestStreamingHealthServices_Filtering(t *testing.T) {
	namespace := getNamespace("ns3")
	client := NewTestStreamingClient(namespace)
	typ := StreamingHealthServices{deps: MaterializerDeps{
		Client: client,
		Logger: hclog.Default(),
	}}

	// Create an initial snapshot of 3 instances but in a single event batch
	batchEv := newEventBatchWithEvents(
		newEventServiceHealthRegister(5, 1, "web"),
		newEventServiceHealthRegister(5, 2, "web"),
		newEventServiceHealthRegister(5, 3, "web"))
	client.QueueEvents(
		batchEv,
		newEndOfSnapshotEvent(5))

	// This contains the view state so important we share it between calls.
	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:     "dc1",
		ServiceName:    "web",
		EnterpriseMeta: structs.NewEnterpriseMeta(namespace),
		QueryOptions: structs.QueryOptions{
			Filter: `Node.Node == "node2"`,
		},
	}

	gatherNodes := func(res interface{}) []string {
		nodes := make([]string, 0, 3)
		r := res.(*structs.IndexedCheckServiceNodes)
		for _, csn := range r.Nodes {
			nodes = append(nodes, csn.Node.Node)
		}
		return nodes
	}

	runStep(t, "filtered snapshot returned", func(t *testing.T) {
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		require.Equal(t, []string{"node2"}, gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})

	runStep(t, "filtered updates work too", func(t *testing.T) {
		// Simulate multiple registrations happening in one Txn (so all have same
		// index)
		batchEv := newEventBatchWithEvents(
			// Deregister an existing node
			newEventServiceHealthDeregister(20, 1, "web"),
			// Register another
			newEventServiceHealthRegister(20, 4, "web"),
		)
		client.QueueEvents(batchEv)
		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(20), result.Index)
		require.Equal(t, []string{"node2"}, gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	})
}

func runStep(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}
*/

func TestNewFilterEvaluator(t *testing.T) {
	type testCase struct {
		name     string
		req      structs.ServiceSpecificRequest
		data     structs.CheckServiceNode
		expected bool
	}

	fn := func(t *testing.T, tc testCase) {
		e, err := newFilterEvaluator(&tc.req)
		require.NoError(t, err)
		actual, err := e.Evaluate(tc.data)
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)
	}

	var testCases = []testCase{
		{
			name: "single ServiceTags match",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"match"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: true,
		},
		{
			name: "single deprecated ServiceTag match",
			req: structs.ServiceSpecificRequest{
				ServiceTag: "match",
				TagFilter:  true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: true,
		},
		{
			name: "single ServiceTags mismatch",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"other"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: false,
		},
		{
			name: "multiple ServiceTags match",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"match", "second"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match", "second"},
				},
			},
			expected: true,
		},
		{
			name: "multiple ServiceTags mismatch",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"match", "not"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: false,
		},
		{
			name: "single NodeMetaFilter match",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{"meta1": "match"},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "match",
						"extra": "some",
					},
				},
			},
			expected: true,
		},
		{
			name: "single NodeMetaFilter mismatch",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{
					"meta1": "match",
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "other",
						"extra": "some",
					},
				},
			},
			expected: false,
		},
		{
			name: "multiple NodeMetaFilter match",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{"meta1": "match", "meta2": "a"},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "match",
						"meta2": "a",
						"extra": "some",
					},
				},
			},
			expected: true,
		},
		{
			name: "multiple NodeMetaFilter mismatch",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{
					"meta1": "match",
					"meta2": "beta",
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "other",
						"meta2": "gamma",
					},
				},
			},
			expected: false,
		},
		{
			name: "QueryOptions.Filter match",
			req: structs.ServiceSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Filter: `Node.Node == "node3"`,
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{Node: "node3"},
			},
			expected: true,
		},
		{
			name: "QueryOptions.Filter mismatch",
			req: structs.ServiceSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Filter: `Node.Node == "node2"`,
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{Node: "node3"},
			},
			expected: false,
		},
		{
			name: "all match",
			req: structs.ServiceSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Filter: `Node.Node == "node3"`,
				},
				ServiceTags: []string{"tag1", "tag2"},
				NodeMetaFilters: map[string]string{
					"meta1": "match1",
					"meta2": "match2",
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Node: "node3",
					Meta: map[string]string{
						"meta1": "match1",
						"meta2": "match2",
						"extra": "other",
					},
				},
				Service: &structs.NodeService{
					Tags: []string{"tag1", "tag2", "extra"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
