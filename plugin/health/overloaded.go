package health

import (
	"net/http"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// overloaded queries the health end point and updates a metrics showing how long it took.
func (h *health) overloaded() {
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	url := "http://" + h.Addr
	tick := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-tick.C:
			start := time.Now()
			resp, err := client.Get(url)
			if err != nil {
				HealthDuration.Observe(timeout.Seconds())
				continue
			}
			resp.Body.Close()
			HealthDuration.Observe(time.Since(start).Seconds())

		case <-h.stop:
			tick.Stop()
			return
		}
	}
}

var (
	// HealthDuration is the metric used for exporting how fast we can retrieve the /health endpoint.
	HealthDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: "health",
		Name:      "request_duration_seconds",
		Buckets:   plugin.TimeBuckets,
		Help:      "Histogram of the time (in seconds) each request took.",
	})
)

var onceMetric sync.Once
