package kubernetes

import (
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/prometheus/client_golang/prometheus"
	api "k8s.io/api/core/v1"
)

const (
	subsystem = "kubernetes"
)

var (
	// DnsProgrammingLatency is defined as the time it took to program a DNS instance - from the time
	// a service or pod has changed to the time the change was propagated and was available to be
	// served by a DNS server.
	// The definition of this SLI can be found at https://github.com/kubernetes/community/blob/master/sig-scalability/slos/dns_programming_latency.md
	// Note that the metrics is partially based on the time exported by the endpoints controller on
	// the master machine. The measurement may be inaccurate if there is a clock drift between the
	// node and master machine.
	// The service_kind label can be one of:
	//   * cluster_ip
	//   * headless_with_selector
	//   * headless_without_selector
	DnsProgrammingLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "dns_programming_duration_seconds",
		// From 1 millisecond to ~17 minutes.
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 20),
		Help:    "Histogram of the time (in seconds) it took to program a dns instance.",
	}, []string{"service_kind"})

	// durationSinceFunc returns the duration elapsed since the given time.
	// Added as a global variable to allow injection for testing.
	durationSinceFunc = time.Since
)

func recordDNSProgrammingLatency(svcs []*object.Service, endpoints *api.Endpoints) {
	// getLastChangeTriggerTime is the time.Time value of the EndpointsLastChangeTriggerTime
	// annotation stored in the given endpoints object or the "zero" time if the annotation wasn't set
	var lastChangeTriggerTime time.Time
	stringVal, ok := endpoints.Annotations[api.EndpointsLastChangeTriggerTime]
	if ok {
		ts, err := time.Parse(time.RFC3339Nano, stringVal)
		if err != nil {
			log.Warningf("DnsProgrammingLatency cannot be calculated for Endpoints '%s/%s'; invalid %q annotation RFC3339 value of %q",
				endpoints.GetNamespace(), endpoints.GetName(), api.EndpointsLastChangeTriggerTime, stringVal)
			// In case of error val = time.Zero, which is ignored in the upstream code.
		}
		lastChangeTriggerTime = ts
	}

	// isHeadless indicates whether the endpoints object belongs to a headless
	// service (i.e. clusterIp = None). Note that this can be a  false negatives if the service
	// informer is lagging, i.e. we may not see a recently created service. Given that the services
	// don't change very often (comparing to much more frequent endpoints changes), cases when this method
	// will return wrong answer should be relatively rare. Because of that we intentionally accept this
	// flaw to keep the solution simple.
	isHeadless := len(svcs) == 1 && svcs[0].ClusterIP == api.ClusterIPNone

	if endpoints == nil || !isHeadless || lastChangeTriggerTime.IsZero() {
		return
	}

	// If we're here it means that the Endpoints object is for a headless service and that
	// the Endpoints object was created by the endpoints-controller (because the
	// LastChangeTriggerTime annotation is set). It means that the corresponding service is a
	// "headless service with selector".
	DnsProgrammingLatency.WithLabelValues("headless_with_selector").
		Observe(durationSinceFunc(lastChangeTriggerTime).Seconds())
}
