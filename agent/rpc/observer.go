package rpc

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
)

type ServiceCallObserver struct {
	Logger hclog.Logger
}

func (i *ServiceCallObserver) Observe(name string, rpcType string, start time.Time, request, response interface{}) {
	errored := "false"
	if _, ok := response.(error); ok {
		errored = "true"
	}
	reqType := requestType(request)

	labels := []metrics.Label{
		{Name: "method", Value: name},
		{Name: "errored", Value: errored},
		{Name: "request_type", Value: reqType},
		{Name: "rpc_type", Value: rpcType},
	}
	metrics.MeasureSinceWithLabels(metricRPCRequest, start, labels)

	i.Logger.Debug(requestLogName,
		"method", name,
		"errored", errored,
		"request_type", reqType,
		"rpc_type", rpcType,
		"elapsed", time.Since(start))
}

func requestType(req interface{}) string {
	if r, ok := req.(interface{ IsRead() bool }); ok && r.IsRead() {
		return "read"
	}
	return "write"
}

var metricRPCRequest = []string{"rpc", "server", "request"}
var requestLogName = "rpc.server.request"
