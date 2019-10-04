package kubernetes

import (
	"strings"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/kubernetes/object"

	"github.com/prometheus/client_golang/prometheus/testutil"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	namespace = "testns"
)

func TestDnsProgrammingLatency(t *testing.T) {
	client := fake.NewSimpleClientset()
	now := time.Now()
	controller := newdnsController(client, dnsControlOpts{
		initEndpointsCache: true,
		// This is needed as otherwise the fake k8s client doesn't work properly.
		skipAPIObjectsCleanup: true,
	})
	durationSinceFunc = func(t time.Time) time.Duration {
		return now.Sub(t)
	}
	DnsProgrammingLatency.Reset()
	go controller.Run()

	subset1 := []api.EndpointSubset{{
		Addresses: []api.EndpointAddress{{IP: "1.2.3.4", Hostname: "foo"}},
	}}

	subset2 := []api.EndpointSubset{{
		Addresses: []api.EndpointAddress{{IP: "1.2.3.5", Hostname: "foo"}},
	}}

	createService(t, client, controller, "my-service", api.ClusterIPNone)
	createEndpoints(t, client, "my-service", now.Add(-2*time.Second), subset1)
	updateEndpoints(t, client, "my-service", now.Add(-1*time.Second), subset2)

	createEndpoints(t, client, "endpoints-no-service", now.Add(-4*time.Second), nil)

	createService(t, client, controller, "clusterIP-service", "10.40.0.12")
	createEndpoints(t, client, "clusterIP-service", now.Add(-8*time.Second), nil)

	createService(t, client, controller, "headless-no-annotation", api.ClusterIPNone)
	createEndpoints(t, client, "headless-no-annotation", nil, nil)

	createService(t, client, controller, "headless-wrong-annotation", api.ClusterIPNone)
	createEndpoints(t, client, "headless-wrong-annotation", "wrong-value", nil)

	controller.Stop()
	expected := `
        # HELP coredns_kubernetes_dns_programming_latency_seconds Histogram of the time (in seconds) it took to program a dns instance.
        # TYPE coredns_kubernetes_dns_programming_latency_seconds histogram
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.001"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.002"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.004"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.008"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.016"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.032"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.064"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.128"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.256"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="0.512"} 0
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="1.024"} 1
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="2.048"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="4.096"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="8.192"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="16.384"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="32.768"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="65.536"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="131.072"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="262.144"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="524.288"} 2
        coredns_kubernetes_dns_programming_latency_seconds_bucket{service_kind="headless_with_selector",le="+Inf"} 2
        coredns_kubernetes_dns_programming_latency_seconds_sum{service_kind="headless_with_selector"} 3
        coredns_kubernetes_dns_programming_latency_seconds_count{service_kind="headless_with_selector"} 2
	`
	if err := testutil.CollectAndCompare(DnsProgrammingLatency, strings.NewReader(expected)); err != nil {
		t.Error(err)
	}
}

func buildEndpoints(name string, lastChangeTriggerTime interface{}, subsets []api.EndpointSubset) *api.Endpoints {
	annotations := make(map[string]string)
	switch v := lastChangeTriggerTime.(type) {
	case string:
		annotations[api.EndpointsLastChangeTriggerTime] = v
	case time.Time:
		annotations[api.EndpointsLastChangeTriggerTime] = v.Format(time.RFC3339Nano)
	}
	return &api.Endpoints{
		ObjectMeta: meta.ObjectMeta{Namespace: namespace, Name: name, Annotations: annotations},
		Subsets:    subsets,
	}
}

func createEndpoints(t *testing.T, client kubernetes.Interface, name string, triggerTime interface{}, subsets []api.EndpointSubset) {
	_, err := client.CoreV1().Endpoints(namespace).Create(buildEndpoints(name, triggerTime, subsets))
	if err != nil {
		t.Fatal(err)
	}
}

func updateEndpoints(t *testing.T, client kubernetes.Interface, name string, triggerTime interface{}, subsets []api.EndpointSubset) {
	_, err := client.CoreV1().Endpoints(namespace).Update(buildEndpoints(name, triggerTime, subsets))
	if err != nil {
		t.Fatal(err)
	}
}

func createService(t *testing.T, client kubernetes.Interface, controller dnsController, name string, clusterIp string) {
	if _, err := client.CoreV1().Services(namespace).Create(&api.Service{
		ObjectMeta: meta.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       api.ServiceSpec{ClusterIP: clusterIp},
	}); err != nil {
		t.Fatal(err)
	}
	if err := wait.PollImmediate(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		return len(controller.SvcIndex(object.ServiceKey(name, namespace))) == 1, nil
	}); err != nil {
		t.Fatal(err)
	}
}
