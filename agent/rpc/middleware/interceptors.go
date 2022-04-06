package middleware

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/go-hclog"
)

// RPCTypeInternal identifies the "RPC" request as coming from some internal
// operation that runs on the cluster leader. Technically this is not an RPC
// request, but these raft.Apply operations have the same impact on blocking
// queries, and streaming subscriptions, so need to be tracked by the same metric
// and logs.
// Really what we are measuring here is a "cluster operation". The term we have
// used for this historically is "RPC", so we continue to use that here.
const RPCTypeInternal = "internal"
const RPCTypeNetRPC = "net/rpc"

var metricRPCRequest = []string{"rpc", "server", "call"}
var requestLogName = strings.Join(metricRPCRequest, "_")

var OneTwelveRPCSummary = []prometheus.SummaryDefinition{
	{
		Name: metricRPCRequest,
		Help: "Measures the time an RPC service call takes to make in milliseconds. Labels mark which RPC method was called and metadata about the call.",
	},
}

type RequestRecorder struct {
	Logger       hclog.Logger
	RecorderFunc func(key []string, val float32, labels []metrics.Label)
}

func NewRequestRecorder(logger hclog.Logger) *RequestRecorder {
	return &RequestRecorder{Logger: logger, RecorderFunc: metrics.AddSampleWithLabels}
}

func (r *RequestRecorder) Record(requestName string, rpcType string, start time.Time, request interface{}, respErrored bool) {
	elapsed := time.Since(start).Milliseconds()
	reqType := requestType(request)

	labels := []metrics.Label{
		{Name: "method", Value: requestName},
		{Name: "errored", Value: strconv.FormatBool(respErrored)},
		{Name: "request_type", Value: reqType},
		{Name: "rpc_type", Value: rpcType},
	}

	// math.MaxInt64 < math.MaxFloat32 is true so we should be good!
	r.RecorderFunc(metricRPCRequest, float32(elapsed), labels)
	r.Logger.Trace(requestLogName,
		"method", requestName,
		"errored", respErrored,
		"request_type", reqType,
		"rpc_type", rpcType,
		"elapsed", elapsed)
}

func requestType(req interface{}) string {
	if r, ok := req.(interface{ IsRead() bool }); ok {
		if r.IsRead() {
			return "read"
		} else {
			return "write"
		}
	}

	// This logical branch should not happen. If it happens
	// it means an underlying request is not implementing the interface.
	// Rather than swallowing it up in a "read" or "write", let's be aware of it.
	return "unreported"
}

func GetNetRPCInterceptor(recorder *RequestRecorder) rpc.ServerServiceCallInterceptor {
	return func(reqServiceMethod string, argv, replyv reflect.Value, handler func() error) {
		reqStart := time.Now()

		err := handler()

		recorder.Record(reqServiceMethod, RPCTypeNetRPC, reqStart, argv.Interface(), err != nil)
	}
}
