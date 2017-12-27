package autopath

import (
	"sync"

	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics for autopath.
var (
	AutoPathCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "autopath",
		Name:      "success_count_total",
		Help:      "Counter of requests that did autopath.",
	}, []string{})
)

var once sync.Once
