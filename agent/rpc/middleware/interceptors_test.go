package middleware

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

// obs holds all the things we want to assert on that we recorded correctly in our tests.
type obs struct {
	key     []string
	elapsed float32
	labels  []metrics.Label
}

// recorderStore acts as an in-mem mock storage for all the RequestRecorder.Record() RecorderFunc calls.
type recorderStore struct {
	lock  sync.Mutex
	store map[string]obs
}

func (rs *recorderStore) put(key []string, o obs) {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	actualKey := strings.Join(append(key, o.labels[0].Value), "")
	rs.store[actualKey] = o
}

func (rs *recorderStore) get(key []string) obs {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	actualKey := strings.Join(key, "")
	return rs.store[actualKey]
}

var store = recorderStore{store: make(map[string]obs)}
var simpleRecorderFunc = func(key []string, val float32, labels []metrics.Label) {
	o := obs{key: key, elapsed: val, labels: labels}
	store.put(key, o)
}

type readRequest struct{}
type writeRequest struct{}

func (rr readRequest) IsRead() bool {
	return true
}

func (wr writeRequest) IsRead() bool {
	return false
}

// TestRequestRecorder_SimpleOK tests that the RequestRecorder can record a simple request.
func TestRequestRecorder_SimpleOK(t *testing.T) {
	t.Parallel()

	r := RequestRecorder{
		Logger:       hclog.NewInterceptLogger(&hclog.LoggerOptions{}),
		RecorderFunc: simpleRecorderFunc,
	}

	start := time.Now()
	r.Record("A.B", RPCTypeInternal, start, struct{}{}, false)

	expectedLabels := []metrics.Label{
		{Name: "method", Value: "A.B"},
		{Name: "errored", Value: "false"},
		{Name: "request_type", Value: "unreported"},
		{Name: "rpc_type", Value: RPCTypeInternal},
	}

	o := store.get(append(metricRPCRequest, expectedLabels[0].Value))
	require.Equal(t, o.key, metricRPCRequest)
	require.LessOrEqual(t, o.elapsed, float32(start.Sub(time.Now()).Milliseconds()))
	require.Equal(t, o.labels, expectedLabels)
}

// TestRequestRecorder_ReadRequest tests that RequestRecorder can record a read request AND a responseErrored arg.
func TestRequestRecorder_ReadRequest(t *testing.T) {
	t.Parallel()

	r := RequestRecorder{
		Logger:       hclog.NewInterceptLogger(&hclog.LoggerOptions{}),
		RecorderFunc: simpleRecorderFunc,
	}

	start := time.Now()

	r.Record("B.A", RPCTypeNetRPC, start, readRequest{}, true)

	expectedLabels := []metrics.Label{
		{Name: "method", Value: "B.A"},
		{Name: "errored", Value: "true"},
		{Name: "request_type", Value: "read"},
		{Name: "rpc_type", Value: RPCTypeNetRPC},
	}

	o := store.get(append(metricRPCRequest, expectedLabels[0].Value))
	require.Equal(t, o.labels, expectedLabels)
}

// TestRequestRecorder_WriteRequest tests that RequestRecorder can record a write request.
func TestRequestRecorder_WriteRequest(t *testing.T) {
	t.Parallel()

	r := RequestRecorder{
		Logger:       hclog.NewInterceptLogger(&hclog.LoggerOptions{}),
		RecorderFunc: simpleRecorderFunc,
	}

	start := time.Now()

	r.Record("B.C", RPCTypeNetRPC, start, writeRequest{}, true)

	expectedLabels := []metrics.Label{
		{Name: "method", Value: "B.C"},
		{Name: "errored", Value: "true"},
		{Name: "request_type", Value: "write"},
		{Name: "rpc_type", Value: RPCTypeNetRPC},
	}

	o := store.get(append(metricRPCRequest, expectedLabels[0].Value))
	require.Equal(t, o.labels, expectedLabels)
}
