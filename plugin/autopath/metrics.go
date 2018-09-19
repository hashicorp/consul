package autopath

import (
	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	autoPathCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "autopath",
		Name:      "success_count_total",
		Help:      "Counter of requests that did autopath.",
	}, []string{"server"})
)
