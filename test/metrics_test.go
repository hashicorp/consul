package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/metrics/vars"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// Start test server that has metrics enabled. Then tear it down again.
func TestMetricsServer(t *testing.T) {
	corefile := `example.org:0 {
	chaos CoreDNS-001 miek@miek.nl
	prometheus localhost:0
}

example.com:0 {
	forward . 8.8.4.4:53
	prometheus localhost:0
}
`
	srv, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer srv.Stop()
}

func TestMetricsRefused(t *testing.T) {
	metricName := "coredns_dns_response_rcode_count_total"

	corefile := `example.org:0 {
	forward . 8.8.8.8:53
	prometheus localhost:0
}
`
	srv, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer srv.Stop()

	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeA)

	if _, err = dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	data := test.Scrape("http://" + metrics.ListenAddr + "/metrics")
	got, labels := test.MetricValue(metricName, data)

	if got != "1" {
		t.Errorf("Expected value %s for refused, but got %s", "1", got)
	}
	if labels["zone"] != vars.Dropped {
		t.Errorf("Expected zone value %s for refused, but got %s", vars.Dropped, labels["zone"])
	}
	if labels["rcode"] != "REFUSED" {
		t.Errorf("Expected zone value %s for refused, but got %s", "REFUSED", labels["rcode"])
	}
}

func TestMetricsAuto(t *testing.T) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "coredns")
	if err != nil {
		t.Fatal(err)
	}

	corefile := `org:0 {
		auto {
			directory ` + tmpdir + ` db\.(.*) {1} 1
		}
		prometheus localhost:0
	}
`

	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}
	defer i.Stop()

	// Write db.example.org to get example.org.
	if err = ioutil.WriteFile(filepath.Join(tmpdir, "db.example.org"), []byte(zoneContent), 0644); err != nil {
		t.Fatal(err)
	}
	// TODO(miek): make the auto sleep even less.
	time.Sleep(1100 * time.Millisecond) // wait for it to be picked up

	m := new(dns.Msg)
	m.SetQuestion("www.example.org.", dns.TypeA)

	if _, err := dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	metricName := "coredns_dns_request_count_total" //{zone, proto, family}

	data := test.Scrape("http://" + metrics.ListenAddr + "/metrics")
	// Get the value for the metrics where the one of the labels values matches "example.org."
	got, _ := test.MetricValueLabel(metricName, "example.org.", data)

	if got != "1" {
		t.Errorf("Expected value %s for %s, but got %s", "1", metricName, got)
	}

	// Remove db.example.org again. And see if the metric stops increasing.
	os.Remove(filepath.Join(tmpdir, "db.example.org"))
	time.Sleep(1100 * time.Millisecond) // wait for it to be picked up
	if _, err := dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	data = test.Scrape("http://" + metrics.ListenAddr + "/metrics")
	got, _ = test.MetricValueLabel(metricName, "example.org.", data)

	if got != "1" {
		t.Errorf("Expected value %s for %s, but got %s", "1", metricName, got)
	}
}

// Show that when 2 blocs share the same metric listener (they have a prometheus plugin on the same listening address),
// ALL the metrics of the second bloc in order are declared in prometheus, especially the plugins that are used ONLY in the second bloc
func TestMetricsSeveralBlocs(t *testing.T) {
	cacheSizeMetricName := "coredns_cache_size"
	addrMetrics := "localhost:9155"

	corefile := fmt.Sprintf(`
example.org:0 {
	prometheus %s
	forward . 8.8.8.8:53 {
       force_tcp
    }
}
google.com:0 {
	prometheus %s
	forward . 8.8.8.8:53 {
       force_tcp
    }
	cache
}
`, addrMetrics, addrMetrics)

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	// send an initial query to setup properly the cache size
	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeA)
	if _, err = dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	beginCacheSize := test.ScrapeMetricAsInt(addrMetrics, cacheSizeMetricName, "", 0)

	// send an query, different from initial to ensure we have another add to the cache
	m = new(dns.Msg)
	m.SetQuestion("www.google.com.", dns.TypeA)

	if _, err = dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	endCacheSize := test.ScrapeMetricAsInt(addrMetrics, cacheSizeMetricName, "", 0)
	if err != nil {
		t.Errorf("Unexpected metric data retrieved for %s : %s", cacheSizeMetricName, err)
	}
	if endCacheSize-beginCacheSize != 1 {
		t.Errorf("Expected metric data retrieved for %s, expected %d, got %d", cacheSizeMetricName, 1, endCacheSize-beginCacheSize)
	}
}
