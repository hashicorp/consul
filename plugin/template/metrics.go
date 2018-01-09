package template

import (
	"sync"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"

	"github.com/mholt/caddy"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics for template.
var (
	TemplateMatchesCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "template",
		Name:      "matches_total",
		Help:      "Counter of template regex matches.",
	}, []string{"zone", "class", "type"})
	TemplateFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "template",
		Name:      "template_failures_total",
		Help:      "Counter of go template failures.",
	}, []string{"zone", "class", "type", "section", "template"})
	TemplateRRFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "template",
		Name:      "rr_failures_total",
		Help:      "Counter of mis-templated RRs.",
	}, []string{"zone", "class", "type", "section", "template"})
)

// OnStartupMetrics sets up the metrics on startup.
func setupMetrics(c *caddy.Controller) error {
	c.OnStartup(func() error {
		metricsOnce.Do(func() {
			m := dnsserver.GetConfig(c).Handler("prometheus")
			if m == nil {
				return
			}
			if x, ok := m.(*metrics.Metrics); ok {
				x.MustRegister(TemplateMatchesCount)
				x.MustRegister(TemplateFailureCount)
				x.MustRegister(TemplateRRFailureCount)
			}
		})
		return nil
	})

	return nil
}

var metricsOnce sync.Once
