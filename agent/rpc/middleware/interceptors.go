// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package middleware

import (
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/consul-net-rpc/net/rpc"
	rpcRate "github.com/hashicorp/consul/agent/consul/rate"
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
	Logger         hclog.Logger
	RecorderFunc   func(key []string, val float32, labels []metrics.Label)
	serverIsLeader func() bool
	localDC        string
}

func NewRequestRecorder(logger hclog.Logger, isLeader func() bool, localDC string) *RequestRecorder {
	return &RequestRecorder{
		Logger:         logger,
		RecorderFunc:   metrics.AddSampleWithLabels,
		serverIsLeader: isLeader,
		localDC:        localDC,
	}
}

func (r *RequestRecorder) Record(requestName string, rpcType string, start time.Time, request interface{}, respErrored bool) {
	elapsed := time.Since(start).Microseconds()
	elapsedMs := float32(elapsed) / 1000
	reqType := requestType(request)
	isLeader := r.getServerLeadership()

	labels := []metrics.Label{
		{Name: "method", Value: requestName},
		{Name: "errored", Value: strconv.FormatBool(respErrored)},
		{Name: "request_type", Value: reqType},
		{Name: "rpc_type", Value: rpcType},
		{Name: "leader", Value: isLeader},
	}

	labels = r.addOptionalLabels(request, labels)

	// math.MaxInt64 < math.MaxFloat32 is true so we should be good!
	r.RecorderFunc(metricRPCRequest, elapsedMs, labels)

	labelsArr := flattenLabels(labels)
	r.Logger.Trace(requestLogName, labelsArr...)

}

func flattenLabels(labels []metrics.Label) []interface{} {

	var labelArr []interface{}
	for _, label := range labels {
		labelArr = append(labelArr, label.Name, label.Value)
	}

	return labelArr
}

func (r *RequestRecorder) addOptionalLabels(request interface{}, labels []metrics.Label) []metrics.Label {
	if rq, ok := request.(readQuery); ok {
		labels = append(labels,
			metrics.Label{
				Name:  "allow_stale",
				Value: strconv.FormatBool(rq.AllowStaleRead()),
			},
			metrics.Label{
				Name:  "blocking",
				Value: strconv.FormatBool(rq.GetMinQueryIndex() > 0),
			})
	}

	if td, ok := request.(targetDC); ok {
		requestDC := td.RequestDatacenter()
		labels = append(labels, metrics.Label{Name: "target_datacenter", Value: requestDC})

		if r.localDC == requestDC {
			labels = append(labels, metrics.Label{Name: "locality", Value: "local"})
		} else {
			labels = append(labels, metrics.Label{Name: "locality", Value: "forwarded"})
		}
	}

	return labels
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

func (r *RequestRecorder) getServerLeadership() string {
	if r.serverIsLeader != nil {
		if r.serverIsLeader() {
			return "true"
		} else {
			return "false"
		}
	}

	// This logical branch should not happen. If it happens
	// it means that we have not plumbed down a way to verify
	// whether the server handling the request was a leader or not
	return "unreported"
}

type readQuery interface {
	GetMinQueryIndex() uint64
	AllowStaleRead() bool
}

type targetDC interface {
	RequestDatacenter() string
}

func GetNetRPCInterceptor(recorder *RequestRecorder) rpc.ServerServiceCallInterceptor {
	return func(reqServiceMethod string, argv, replyv reflect.Value, handler func() error) {
		reqStart := time.Now()

		err := handler()

		recorder.Record(reqServiceMethod, RPCTypeNetRPC, reqStart, argv.Interface(), err != nil)
	}
}

func GetNetRPCRateLimitingInterceptor(requestLimitsHandler rpcRate.RequestLimitsHandler, panicHandler RecoveryHandlerFunc) rpc.PreBodyInterceptor {

	return func(reqServiceMethod string, sourceAddr net.Addr) (retErr error) {

		defer func() {
			if r := recover(); r != nil {
				retErr = panicHandler(r)
			}
		}()

		op := rpcRate.Operation{
			Name:       reqServiceMethod,
			SourceAddr: sourceAddr,
			Type:       rpcRateLimitSpecs[reqServiceMethod].Type,
			Category:   rpcRateLimitSpecs[reqServiceMethod].Category,
		}

		// net/rpc does not provide a way to encode the nuances of the
		// error response (retry or retry elsewhere) so the error string
		// from the rate limiter is all that we have.
		return requestLimitsHandler.Allow(op)
	}
}
