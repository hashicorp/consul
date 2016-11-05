package test

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/miekg/coredns/middleware/cache"
	"github.com/miekg/coredns/middleware/metrics"
	mtest "github.com/miekg/coredns/middleware/metrics/test"
	"github.com/miekg/coredns/middleware/metrics/vars"

	"github.com/miekg/dns"
)

// Start test server that has metrics enabled. Then tear it down again.
func TestMetricsServer(t *testing.T) {
	corefile := `example.org:0 {
	chaos CoreDNS-001 miek@miek.nl
	prometheus localhost:0
}

example.com:0 {
	proxy . 8.8.4.4:53
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
	proxy . 8.8.8.8:53
	prometheus localhost:0
}
`
	srv, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer srv.Stop()

	udp, _ := CoreDNSServerPorts(srv, 0)

	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeA)

	if _, err = dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	data := mtest.Scrape(t, "http://"+metrics.ListenAddr+"/metrics")
	got, labels := mtest.MetricValue(metricName, data)

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

// TODO(miek): disabled for now - fails in weird ways in travis.
func testMetricsCache(t *testing.T) {
	cacheSizeMetricName := "coredns_cache_size"
	cacheHitMetricName := "coredns_cache_hits_total"

	corefile := `www.example.net:0 {
	proxy . 8.8.8.8:53
	prometheus localhost:0
	cache
}
`
	srv, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer srv.Stop()

	udp, _ := CoreDNSServerPorts(srv, 0)

	m := new(dns.Msg)
	m.SetQuestion("www.example.net.", dns.TypeA)

	if _, err = dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	data := mtest.Scrape(t, "http://"+metrics.ListenAddr+"/metrics")
	// Get the value for the cache size metric where the one of the labels values matches "success".
	got, _ := mtest.MetricValueLabel(cacheSizeMetricName, cache.Success, data)

	if got != "1" {
		t.Errorf("Expected value %s for %s, but got %s", "1", cacheSizeMetricName, got)
	}

	// Second request for the same response to test hit counter.
	if _, err = dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	data = mtest.Scrape(t, "http://"+metrics.ListenAddr+"/metrics")
	// Get the value for the cache hit counter where the one of the labels values matches "success".
	got, _ = mtest.MetricValueLabel(cacheHitMetricName, cache.Success, data)

	if got != "2" {
		t.Errorf("Expected value %s for %s, but got %s", "2", cacheHitMetricName, got)
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

	log.SetOutput(ioutil.Discard)

	// Write db.example.org to get example.org.
	if err = ioutil.WriteFile(path.Join(tmpdir, "db.example.org"), []byte(zoneContent), 0644); err != nil {
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

	data := mtest.Scrape(t, "http://"+metrics.ListenAddr+"/metrics")
	// Get the value for the metrics where the one of the labels values matches "example.org."
	got, _ := mtest.MetricValueLabel(metricName, "example.org.", data)

	if got != "1" {
		t.Errorf("Expected value %s for %s, but got %s", "1", metricName, got)
	}

	// Remove db.example.org again. And see if the metric stops increasing.
	os.Remove(path.Join(tmpdir, "db.example.org"))
	time.Sleep(1100 * time.Millisecond) // wait for it to be picked up
	if _, err := dns.Exchange(m, udp); err != nil {
		t.Fatalf("Could not send message: %s", err)
	}

	data = mtest.Scrape(t, "http://"+metrics.ListenAddr+"/metrics")
	got, _ = mtest.MetricValueLabel(metricName, "example.org.", data)

	if got != "1" {
		t.Errorf("Expected value %s for %s, but got %s", "1", metricName, got)
	}
}
