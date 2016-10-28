package metrics

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	mtest "github.com/miekg/coredns/middleware/metrics/test"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestMetrics(t *testing.T) {
	met := &Metrics{Addr: "localhost:0", zoneMap: make(map[string]bool)}
	if err := met.OnStartup(); err != nil {
		t.Fatalf("Failed to start metrics handler: %s", err)
	}
	defer met.OnShutdown()

	met.AddZone("example.org.")

	tests := []struct {
		next          middleware.Handler
		qname         string
		qtype         uint16
		metric        string
		expectedValue string
	}{
		// This all works because 1 bucket (1 zone, 1 type)
		{
			next:          test.NextHandler(dns.RcodeSuccess, nil),
			qname:         "example.org",
			metric:        "coredns_dns_request_count_total",
			expectedValue: "1",
		},
		{
			next:          test.NextHandler(dns.RcodeSuccess, nil),
			qname:         "example.org",
			metric:        "coredns_dns_request_count_total",
			expectedValue: "2",
		},
		{
			next:          test.NextHandler(dns.RcodeSuccess, nil),
			qname:         "example.org",
			metric:        "coredns_dns_request_type_count_total",
			expectedValue: "3",
		},
		{
			next:          test.NextHandler(dns.RcodeSuccess, nil),
			qname:         "example.org",
			metric:        "coredns_dns_response_rcode_count_total",
			expectedValue: "4",
		},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		if tc.qtype == 0 {
			tc.qtype = dns.TypeA
		}
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)
		met.Next = tc.next

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := met.ServeDNS(ctx, rec, req)
		if err != nil {
			t.Fatalf("Test %d: Expected no error, but got %s", i, err)
		}

		result := mtest.Scrape(t, "http://"+ListenAddr+"/metrics")

		if tc.expectedValue != "" {
			got, _ := mtest.MetricValue(tc.metric, result)
			if got != tc.expectedValue {
				t.Errorf("Test %d: Expected value %s for metrics %s, but got %s", i, tc.expectedValue, tc.metric, got)
			}
		}
	}
}
