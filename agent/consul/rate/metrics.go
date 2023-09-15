package rate

import "github.com/armon/go-metrics/prometheus"

var Counters = []prometheus.CounterDefinition{
	{
		Name: []string{"rpc", "rate_limit", "exceeded"},
		Help: "Increments whenever an RPC is over a configured rate limit. Note: in permissive mode, the RPC will have still been allowed to proceed.",
	},
	{
		Name: []string{"rpc", "rate_limit", "log_dropped"},
		Help: "Increments whenever a log that is emitted because an RPC exceeded a rate limit gets dropped because the output buffer is full.",
	},
}
