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
type readReqWithTD struct{}
type writeReqWithTD struct{}

func (rr readRequest) IsRead() bool {
	return true
}

func (wr writeRequest) IsRead() bool {
	return false
}

func (r readReqWithTD) IsRead() bool {
	return true
}

func (r readReqWithTD) RequestDatacenter() string {
	return "dc3"
}

func (r readReqWithTD) GetMinQueryIndex() uint64 {
	return 1
}
func (r readReqWithTD) AllowStaleRead() bool {
	return false
}

func (w writeReqWithTD) IsRead() bool {
	return false
}

func (w writeReqWithTD) RequestDatacenter() string {
	return "dc2"
}

type testCase struct {
	name string
	// description is meant for human friendliness
	description string
	// requestName is encouraged to be unique across tests to
	// avoid lock contention
	requestName string
	requestI    interface{}
	rpcType     string
	errored     bool
	isLeader    func() bool
	dc          string
	// the first element in expectedLabels should be the method name
	expectedLabels []metrics.Label
}

var testCases = []testCase{
	{
		name:        "simple ok",
		description: "This is a simple happy path test case. We check for pass through and normal request processing",
		requestName: "A.B",
		requestI:    struct{}{},
		rpcType:     RPCTypeInternal,
		errored:     false,
		dc:          "dc1",
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "A.B"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "unreported"},
			{Name: "rpc_type", Value: RPCTypeInternal},
			{Name: "leader", Value: "unreported"},
		},
	},
	{
		name:        "simple ok errored",
		description: "Checks that the errored value is populated right.",
		requestName: "A.C",
		requestI:    struct{}{},
		rpcType:     "test",
		errored:     true,
		dc:          "dc1",
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "A.C"},
			{Name: "errored", Value: "true"},
			{Name: "request_type", Value: "unreported"},
			{Name: "rpc_type", Value: "test"},
			{Name: "leader", Value: "unreported"},
		},
	},
	{
		name:        "read request, rpc type internal",
		description: "Checks for read request interface parsing",
		requestName: "B.C",
		requestI:    readRequest{},
		rpcType:     RPCTypeInternal,
		errored:     false,
		dc:          "dc1",
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "B.C"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "read"},
			{Name: "rpc_type", Value: RPCTypeInternal},
			{Name: "leader", Value: "unreported"},
		},
	},
	{
		name:        "write request, rpc type net/rpc",
		description: "Checks for write request interface, different RPC type",
		requestName: "D.E",
		requestI:    writeRequest{},
		rpcType:     RPCTypeNetRPC,
		errored:     false,
		dc:          "dc1",
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "D.E"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "write"},
			{Name: "rpc_type", Value: RPCTypeNetRPC},
			{Name: "leader", Value: "unreported"},
		},
	},
	{
		name:        "read request with blocking stale and target dc",
		description: "Checks for locality, blocking status and target dc",
		requestName: "E.F",
		requestI:    readReqWithTD{},
		rpcType:     RPCTypeNetRPC,
		errored:     false,
		dc:          "dc1",
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "E.F"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "read"},
			{Name: "rpc_type", Value: RPCTypeNetRPC},
			{Name: "leader", Value: "unreported"},
			{Name: "allow_stale", Value: "false"},
			{Name: "blocking", Value: "true"},
			{Name: "target_datacenter", Value: "dc3"},
			{Name: "locality", Value: "forwarded"},
		},
	},
	{
		name:        "write request with TD, locality local",
		description: "Checks for write request with local forwarding and target dc",
		requestName: "F.G",
		requestI:    writeReqWithTD{},
		rpcType:     RPCTypeNetRPC,
		errored:     false,
		dc:          "dc2",
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "F.G"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "write"},
			{Name: "rpc_type", Value: RPCTypeNetRPC},
			{Name: "leader", Value: "unreported"},
			{Name: "target_datacenter", Value: "dc2"},
			{Name: "locality", Value: "local"},
		},
	},
	{
		name:        "is leader",
		description: "checks for is leader",
		requestName: "G.H",
		requestI:    struct{}{},
		rpcType:     "test",
		errored:     false,
		isLeader: func() bool {
			return true
		},
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "G.H"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "unreported"},
			{Name: "rpc_type", Value: "test"},
			{Name: "leader", Value: "true"},
		},
	},
	{
		name:        "is not leader",
		description: "checks for is not leader",
		requestName: "H.I",
		requestI:    struct{}{},
		rpcType:     "test",
		errored:     false,
		isLeader: func() bool {
			return false
		},
		expectedLabels: []metrics.Label{
			{Name: "method", Value: "H.I"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "unreported"},
			{Name: "rpc_type", Value: "test"},
			{Name: "leader", Value: "false"},
		},
	},
}

// TestRequestRecorder goes over all the parsing and reporting that RequestRecorder
// is expected to perform.
func TestRequestRecorder(t *testing.T) {

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			r := RequestRecorder{
				Logger:         hclog.NewInterceptLogger(&hclog.LoggerOptions{}),
				RecorderFunc:   simpleRecorderFunc,
				serverIsLeader: tc.isLeader,
				localDC:        tc.dc,
			}

			start := time.Now()
			r.Record(tc.requestName, tc.rpcType, start, tc.requestI, tc.errored)

			key := append(metricRPCRequest, tc.expectedLabels[0].Value)
			o := store.get(key)

			require.Equal(t, o.key, metricRPCRequest)
			require.LessOrEqual(t, o.elapsed, float32(start.Sub(time.Now()).Milliseconds()))
			require.Equal(t, o.labels, tc.expectedLabels)

		})
	}
}
