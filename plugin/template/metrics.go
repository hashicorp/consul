package template

import (
	"sync"

	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics for template.
var (
	TemplateMatchesCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "template",
		Name:      "matches_total",
		Help:      "Counter of template regex matches.",
	}, []string{"regex"})
	TemplateFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "template",
		Name:      "template_failures_total",
		Help:      "Counter of go template failures.",
	}, []string{"regex", "section", "template"})
	TemplateRRFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "template",
		Name:      "rr_failures_total",
		Help:      "Counter of mis-templated RRs.",
	}, []string{"regex", "section", "template"})
)

// OnStartupMetrics sets up the metrics on startup.
func OnStartupMetrics() error {
	metricsOnce.Do(func() {
		prometheus.MustRegister(TemplateMatchesCount)
		prometheus.MustRegister(TemplateFailureCount)
		prometheus.MustRegister(TemplateRRFailureCount)
	})
	return nil
}

var metricsOnce sync.Once
