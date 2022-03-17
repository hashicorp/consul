package middleware

import (
	"reflect"
	"strconv"
	"time"

	"github.com/armon/go-metrics"
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

var metricRPCRequest = []string{"rpc", "server", "request"}
var requestLogName = "rpc.server.request"

type RequestRecorder struct {
	Logger hclog.Logger
}

func NewRequestRecorder(logger hclog.Logger) *RequestRecorder {
	return &RequestRecorder{Logger: logger}
}

func (r *RequestRecorder) Record(requestName string, rpcType string, start time.Time, request interface{}, respErrored bool) {
	elapsed := time.Since(start)

	reqType := requestType(request)

	labels := []metrics.Label{
		{Name: "method", Value: requestName},
		{Name: "errored", Value: strconv.FormatBool(respErrored)},
		{Name: "request_type", Value: reqType},
		{Name: "rpc_type", Value: rpcType},
	}

	// TODO(rpc-metrics-improv): consider using Telemetry API call here
	// It'd be neat if we could actually pass the elapsed observed above
	metrics.MeasureSinceWithLabels(metricRPCRequest, start, labels)

	r.Logger.Debug(requestLogName,
		"method", requestName,
		"errored", respErrored,
		"request_type", reqType,
		"rpc_type", rpcType,
		"elapsed", elapsed)
}

func requestType(req interface{}) string {
	if r, ok := req.(interface{ IsRead() bool }); ok && r.IsRead() {
		return "read"
	}
	return "write"
}

func GetNetRPCInterceptor(recorder *RequestRecorder) rpc.ServerServiceCallInterceptor {
	return func(reqServiceMethod string, argv, replyv reflect.Value, handler func() error) {
		reqStart := time.Now()

		err := handler()

		responseErr := false
		if err != nil {
			responseErr = true
		}

		recorder.Record(reqServiceMethod, "net/rpc", reqStart, argv.Interface(), responseErr)
	}
}
