package cachetype

import (
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"

	"github.com/stretchr/testify/require"
)

func makeHealthEvent(t *testing.T, idx uint64, nodeIdx int, svc string, register bool) *stream.Event {
	op := stream.CatalogOp_Register
	if !register {
		op = stream.CatalogOp_Deregister
	}
	return &stream.Event{
		Topic: stream.Topic_ServiceHealth,
		Index: idx,
		Payload: &stream.Event_ServiceHealth{
			ServiceHealth: &stream.ServiceHealthUpdate{
				Op: op,
				CheckServiceNode: &stream.CheckServiceNode{
					Node: &stream.Node{
						Node:    fmt.Sprintf("node%03d", nodeIdx),
						Address: fmt.Sprintf("10.0.0.%d", nodeIdx),
					},
					Service: &stream.NodeService{
						Service: svc,
						Port:    8080,
					},
				},
			},
		},
	}
}

func makeBatchEvent(t *testing.T, idx uint64, events ...*stream.Event) *stream.Event {
	return &stream.Event{
		Topic: stream.Topic_ServiceHealth,
		Index: idx,
		Payload: &stream.Event_EventBatch{EventBatch: &stream.EventBatch{
			Events: events,
		}},
	}
}

func makeEOSEvent(t *testing.T, idx uint64) *stream.Event {
	return &stream.Event{
		Topic:   stream.Topic_ServiceHealth,
		Index:   idx,
		Payload: &stream.Event_EndOfSnapshot{EndOfSnapshot: true},
	}
}

func makeResumeEvent(t *testing.T, idx uint64) *stream.Event {
	return &stream.Event{
		Topic:   stream.Topic_ServiceHealth,
		Index:   idx,
		Payload: &stream.Event_ResumeStream{ResumeStream: true},
	}
}

func makeReloadEvent(t *testing.T, idx uint64) *stream.Event {
	return &stream.Event{
		Topic:   stream.Topic_ServiceHealth,
		Index:   idx,
		Payload: &stream.Event_ReloadStream{ReloadStream: true},
	}
}

func TestStreamingHealthServices_EmptySnapshot(t *testing.T) {
	client := NewTestStreamingClient()
	typ := StreamingHealthServices{
		client: client,
		logger: log.New(os.Stderr, "test", log.LstdFlags),
	}

	// Initially there are no services registered. Server should send an
	// EndOfSnapshot message immediately with index of 1.
	client.QueueEvents(makeEOSEvent(t, 1))

	// This contains the view state so important we share it between calls.
	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	}
	empty := &structs.IndexedCheckServiceNodes{
		Nodes: structs.CheckServiceNodes{},
		QueryMeta: structs.QueryMeta{
			Index: 1,
		},
	}

	require.True(t, t.Run("empty snapshot returned", func(t *testing.T) {
		// Fetch should return an empty
		// result of the right type with a non-zero index, and in the background begin
		// streaming updates.
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(1), result.Index)
		require.Equal(t, empty, result.Value)

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))

	require.True(t, t.Run("blocks for timeout", func(t *testing.T) {
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
	}))

	require.True(t, t.Run("blocks until update", func(t *testing.T) {
		// Make another blocking query with a longer timeout and trigger an update
		// event part way through.
		start := time.Now()
		go func() {
			time.Sleep(200 * time.Millisecond)

			// Then a service registers
			client.QueueEvents(makeHealthEvent(t, 4, 1, "web", true))
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
	}))

	require.True(t, t.Run("reconnects and resumes after stream error", func(t *testing.T) {
		client.QueueErr(errors.New("broken pipe"))

		// After the error the view should re-subscribe with same index so will get
		// a "resume stream".
		client.QueueEvents(makeResumeEvent(t, opts.MinIndex))

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
		client.QueueEvents(makeHealthEvent(t, 10, 2, "web", true))

		opts.Timeout = time.Second
		result, err = typ.Fetch(opts, req)
		require.NoError(t, err)
		elapsed = time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until the event was delivered")
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(10), result.Index, "result index should not have changed")
		require.Len(t, result.Value.(*structs.IndexedCheckServiceNodes).Nodes, 2,
			"result value should contain the new registration")

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))
}

func TestStreamingHealthServices_FullSnapshot(t *testing.T) {
	client := NewTestStreamingClient()
	typ := StreamingHealthServices{
		client: client,
		logger: log.New(os.Stderr, "test", log.LstdFlags),
	}

	// Create an initial snapshot of 3 instances on different nodes
	client.QueueEvents(
		makeHealthEvent(t, 5, 1, "web", true),
		makeHealthEvent(t, 5, 2, "web", true),
		makeHealthEvent(t, 5, 3, "web", true),
		makeEOSEvent(t, 5),
	)

	// This contains the view state so important we share it between calls.
	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	}

	gatherNodes := func(res interface{}) []string {
		nodes := make([]string, 0, 3)
		r := res.(*structs.IndexedCheckServiceNodes)
		for _, csn := range r.Nodes {
			nodes = append(nodes, csn.Node.Node)
		}
		return nodes
	}

	require.True(t, t.Run("full snapshot returned", func(t *testing.T) {
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		require.ElementsMatch(t, []string{"node001", "node002", "node003"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))

	require.True(t, t.Run("blocks until deregistration", func(t *testing.T) {
		// Make another blocking query with a longer timeout and trigger an update
		// event part way through.
		start := time.Now()
		go func() {
			time.Sleep(200 * time.Millisecond)

			// Deregister instance on node001
			client.QueueEvents(makeHealthEvent(t, 20, 1, "web", false))
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
		require.ElementsMatch(t, []string{"node002", "node003"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))

	require.True(t, t.Run("server reload is respected", func(t *testing.T) {
		// Simulates the server noticing the request's ACL token privs changing. To
		// detect this we'll queue up the new snapshot as a different set of nodes
		// to the first.
		client.QueueEvents(
			makeReloadEvent(t, 45),
			makeHealthEvent(t, 50, 3, "web", true), // overlap same node
			makeHealthEvent(t, 50, 4, "web", true),
			makeHealthEvent(t, 50, 5, "web", true),
			makeEOSEvent(t, 50),
		)

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
		require.ElementsMatch(t, []string{"node003", "node004", "node005"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))
}

func TestStreamingHealthServices_EventBatches(t *testing.T) {
	client := NewTestStreamingClient()
	typ := StreamingHealthServices{
		client: client,
		logger: log.New(os.Stderr, "test", log.LstdFlags),
	}

	// Create an initial snapshot of 3 instances but in a single event batch
	client.QueueEvents(
		makeBatchEvent(t, 5,
			makeHealthEvent(t, 10, 1, "web", true),
			makeHealthEvent(t, 10, 2, "web", true),
			makeHealthEvent(t, 10, 3, "web", true),
		),
		makeEOSEvent(t, 5),
	)

	// This contains the view state so important we share it between calls.
	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	}

	gatherNodes := func(res interface{}) []string {
		nodes := make([]string, 0, 3)
		r := res.(*structs.IndexedCheckServiceNodes)
		for _, csn := range r.Nodes {
			nodes = append(nodes, csn.Node.Node)
		}
		return nodes
	}

	require.True(t, t.Run("full snapshot returned", func(t *testing.T) {
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		require.ElementsMatch(t, []string{"node001", "node002", "node003"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))

	require.True(t, t.Run("batched updates work too", func(t *testing.T) {
		// Simulate multiple registrations happening in one Txn (so all have same
		// index)
		client.QueueEvents(makeBatchEvent(t, 20,
			// Deregister an existing node
			makeHealthEvent(t, 20, 1, "web", false),
			// Register another
			makeHealthEvent(t, 30, 4, "web", true),
		))
		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(20), result.Index)
		require.ElementsMatch(t, []string{"node002", "node003", "node004"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))
}

func TestStreamingHealthServices_Filtering(t *testing.T) {
	client := NewTestStreamingClient()
	typ := StreamingHealthServices{
		client: client,
		logger: log.New(os.Stderr, "test", log.LstdFlags),
	}

	// Create an initial snapshot of 3 instances but in a single event batch
	client.QueueEvents(
		makeBatchEvent(t, 5,
			makeHealthEvent(t, 5, 1, "web", true),
			makeHealthEvent(t, 5, 2, "web", true),
			makeHealthEvent(t, 5, 3, "web", true),
		),
		makeEOSEvent(t, 5),
	)

	// This contains the view state so important we share it between calls.
	opts := cache.FetchOptions{
		MinIndex: 0,
		Timeout:  1 * time.Second,
	}
	req := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
		QueryOptions: structs.QueryOptions{
			Filter: `Node.Node == "node002"`,
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

	require.True(t, t.Run("filtered snapshot returned", func(t *testing.T) {
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		require.ElementsMatch(t, []string{"node002"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))

	require.True(t, t.Run("filtered updates work too", func(t *testing.T) {
		// Simulate multiple registrations happening in one Txn (so all have same
		// index)
		client.QueueEvents(makeBatchEvent(t, 20,
			// Deregister an existing node
			makeHealthEvent(t, 20, 1, "web", false),
			// Register another
			makeHealthEvent(t, 30, 4, "web", true),
		))
		opts.Timeout = time.Second
		result, err := typ.Fetch(opts, req)
		require.NoError(t, err)

		require.Equal(t, uint64(20), result.Index)
		require.ElementsMatch(t, []string{"node002"},
			gatherNodes(result.Value))

		opts.MinIndex = result.Index
		opts.LastResult = &result
	}))
}
