package clouddns

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/plugin/test"
	crequest "github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	gcp "google.golang.org/api/dns/v1"
)

type fakeGCPClient struct {
	*gcp.Service
}

func (c fakeGCPClient) zoneExists(projectName, hostedZoneName string) error {
	return nil
}

func (c fakeGCPClient) listRRSets(projectName, hostedZoneName string) (*gcp.ResourceRecordSetsListResponse, error) {
	if projectName == "bad-project" || hostedZoneName == "bad-zone" {
		return nil, errors.New("the 'parameters.managedZone' resource named 'bad-zone' does not exist")
	}

	var rr []*gcp.ResourceRecordSet

	if hostedZoneName == "sample-zone-1" {
		rr = []*gcp.ResourceRecordSet{
			{
				Name:    "example.org.",
				Ttl:     300,
				Type:    "A",
				Rrdatas: []string{"1.2.3.4"},
			},
			{
				Name:    "www.example.org",
				Ttl:     300,
				Type:    "A",
				Rrdatas: []string{"1.2.3.4"},
			},
			{
				Name:    "*.www.example.org",
				Ttl:     300,
				Type:    "CNAME",
				Rrdatas: []string{"www.example.org"},
			},
			{
				Name:    "example.org.",
				Ttl:     300,
				Type:    "AAAA",
				Rrdatas: []string{"2001:db8:85a3::8a2e:370:7334"},
			},
			{
				Name:    "sample.example.org",
				Ttl:     300,
				Type:    "CNAME",
				Rrdatas: []string{"example.org"},
			},
			{
				Name:    "example.org.",
				Ttl:     300,
				Type:    "PTR",
				Rrdatas: []string{"ptr.example.org."},
			},
			{
				Name:    "org.",
				Ttl:     300,
				Type:    "SOA",
				Rrdatas: []string{"ns-cloud-c1.googledomains.com. cloud-dns-hostmaster.google.com. 1 21600 300 259200 300"},
			},
			{
				Name:    "com.",
				Ttl:     300,
				Type:    "NS",
				Rrdatas: []string{"ns-cloud-c4.googledomains.com."},
			},
			{
				Name:    "split-example.gov.",
				Ttl:     300,
				Type:    "A",
				Rrdatas: []string{"1.2.3.4"},
			},
			{
				Name:    "swag.",
				Ttl:     300,
				Type:    "YOLO",
				Rrdatas: []string{"foobar"},
			},
		}
	} else {
		rr = []*gcp.ResourceRecordSet{
			{
				Name:    "split-example.org.",
				Ttl:     300,
				Type:    "A",
				Rrdatas: []string{"1.2.3.4"},
			},
			{
				Name:    "other-example.org.",
				Ttl:     300,
				Type:    "A",
				Rrdatas: []string{"3.5.7.9"},
			},
			{
				Name:    "org.",
				Ttl:     300,
				Type:    "SOA",
				Rrdatas: []string{"ns-cloud-e1.googledomains.com. cloud-dns-hostmaster.google.com. 1 21600 300 259200 300"},
			},
		}
	}

	return &gcp.ResourceRecordSetsListResponse{Rrsets: rr}, nil
}

func TestCloudDNS(t *testing.T) {
	ctx := context.Background()

	r, err := New(ctx, fakeGCPClient{}, map[string][]string{"bad.": {"bad-project:bad-zone"}}, &upstream.Upstream{})
	if err != nil {
		t.Fatalf("Failed to create Cloud DNS: %v", err)
	}
	if err = r.Run(ctx); err == nil {
		t.Fatalf("Expected errors for zone bad.")
	}

	r, err = New(ctx, fakeGCPClient{}, map[string][]string{"org.": {"sample-project-1:sample-zone-2", "sample-project-1:sample-zone-1"}, "gov.": {"sample-project-1:sample-zone-2", "sample-project-1:sample-zone-1"}}, &upstream.Upstream{})
	if err != nil {
		t.Fatalf("Failed to create Cloud DNS: %v", err)
	}
	r.Fall = fall.Zero
	r.Fall.SetZonesFromArgs([]string{"gov."})
	r.Next = test.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		state := crequest.Request{W: w, Req: r}
		qname := state.Name()
		m := new(dns.Msg)
		rcode := dns.RcodeServerFailure
		if qname == "example.gov." {
			m.SetReply(r)
			rr, err := dns.NewRR("example.gov.  300 IN  A   2.4.6.8")
			if err != nil {
				t.Fatalf("Failed to create Resource Record: %v", err)
			}
			m.Answer = []dns.RR{rr}

			m.Authoritative = true
			rcode = dns.RcodeSuccess

		}

		m.SetRcode(r, rcode)
		w.WriteMsg(m)
		return rcode, nil
	})
	err = r.Run(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize Cloud DNS: %v", err)
	}

	tests := []struct {
		qname        string
		qtype        uint16
		wantRetCode  int
		wantAnswer   []string // ownernames for the records in the additional section.
		wantMsgRCode int
		wantNS       []string
		expectedErr  error
	}{
		// 0. example.org A found - success.
		{
			qname: "example.org",
			qtype: dns.TypeA,
			wantAnswer: []string{"example.org.	300	IN	A	1.2.3.4"},
		},
		// 1. example.org AAAA found - success.
		{
			qname: "example.org",
			qtype: dns.TypeAAAA,
			wantAnswer: []string{"example.org.	300	IN	AAAA	2001:db8:85a3::8a2e:370:7334"},
		},
		// 2. exampled.org PTR found - success.
		{
			qname: "example.org",
			qtype: dns.TypePTR,
			wantAnswer: []string{"example.org.	300	IN	PTR	ptr.example.org."},
		},
		// 3. sample.example.org points to example.org CNAME.
		// Query must return both CNAME and A recs.
		{
			qname: "sample.example.org",
			qtype: dns.TypeA,
			wantAnswer: []string{
				"sample.example.org.	300	IN	CNAME	example.org.",
				"example.org.	300	IN	A	1.2.3.4",
			},
		},
		// 4. Explicit CNAME query for sample.example.org.
		// Query must return just CNAME.
		{
			qname: "sample.example.org",
			qtype: dns.TypeCNAME,
			wantAnswer: []string{"sample.example.org.	300	IN	CNAME	example.org."},
		},
		// 5. Explicit SOA query for example.org.
		{
			qname: "example.org",
			qtype: dns.TypeSOA,
			wantAnswer: []string{"org.	300	IN	SOA	ns-cloud-e1.googledomains.com. cloud-dns-hostmaster.google.com. 1 21600 300 259200 300"},
		},
		// 6. Explicit SOA query for example.org.
		{
			qname: "example.org",
			qtype: dns.TypeNS,
			wantNS: []string{"org.	300	IN	SOA	ns-cloud-c1.googledomains.com. cloud-dns-hostmaster.google.com. 1 21600 300 259200 300"},
		},
		// 7. AAAA query for split-example.org must return NODATA.
		{
			qname:       "split-example.gov",
			qtype:       dns.TypeAAAA,
			wantRetCode: dns.RcodeSuccess,
			wantNS: []string{"org.	300	IN	SOA	ns-cloud-c1.googledomains.com. cloud-dns-hostmaster.google.com. 1 21600 300 259200 300"},
		},
		// 8. Zone not configured.
		{
			qname:        "badexample.com",
			qtype:        dns.TypeA,
			wantRetCode:  dns.RcodeServerFailure,
			wantMsgRCode: dns.RcodeServerFailure,
		},
		// 9. No record found. Return SOA record.
		{
			qname:        "bad.org",
			qtype:        dns.TypeA,
			wantRetCode:  dns.RcodeSuccess,
			wantMsgRCode: dns.RcodeNameError,
			wantNS: []string{"org.	300	IN	SOA	ns-cloud-c1.googledomains.com. cloud-dns-hostmaster.google.com. 1 21600 300 259200 300"},
		},
		// 10. No record found. Fallthrough.
		{
			qname: "example.gov",
			qtype: dns.TypeA,
			wantAnswer: []string{"example.gov.	300	IN	A	2.4.6.8"},
		},
		// 11. other-zone.example.org is stored in a different hosted zone. success
		{
			qname: "other-example.org",
			qtype: dns.TypeA,
			wantAnswer: []string{"other-example.org.	300	IN	A	3.5.7.9"},
		},
		// 12. split-example.org only has A record. Expect NODATA.
		{
			qname: "split-example.org",
			qtype: dns.TypeAAAA,
			wantNS: []string{"org.	300	IN	SOA	ns-cloud-e1.googledomains.com. cloud-dns-hostmaster.google.com. 1 21600 300 259200 300"},
		},
		// 13. *.www.example.org is a wildcard CNAME to www.example.org.
		{
			qname: "a.www.example.org",
			qtype: dns.TypeA,
			wantAnswer: []string{
				"a.www.example.org.	300	IN	CNAME	www.example.org.",
				"www.example.org.	300	IN	A	1.2.3.4",
			},
		},
	}

	for ti, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := r.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Fatalf("Test %d: Expected error %v, but got %v", ti, tc.expectedErr, err)
		}
		if code != int(tc.wantRetCode) {
			t.Fatalf("Test %d: Expected returned status code %s, but got %s", ti, dns.RcodeToString[tc.wantRetCode], dns.RcodeToString[code])
		}

		if tc.wantMsgRCode != rec.Msg.Rcode {
			t.Errorf("Test %d: Unexpected msg status code. Want: %s, got: %s", ti, dns.RcodeToString[tc.wantMsgRCode], dns.RcodeToString[rec.Msg.Rcode])
		}

		if len(tc.wantAnswer) != len(rec.Msg.Answer) {
			t.Errorf("Test %d: Unexpected number of Answers. Want: %d, got: %d", ti, len(tc.wantAnswer), len(rec.Msg.Answer))
		} else {
			for i, gotAnswer := range rec.Msg.Answer {
				if gotAnswer.String() != tc.wantAnswer[i] {
					t.Errorf("Test %d: Unexpected answer.\nWant:\n\t%s\nGot:\n\t%s", ti, tc.wantAnswer[i], gotAnswer)
				}
			}
		}

		if len(tc.wantNS) != len(rec.Msg.Ns) {
			t.Errorf("Test %d: Unexpected NS number. Want: %d, got: %d", ti, len(tc.wantNS), len(rec.Msg.Ns))
		} else {
			for i, ns := range rec.Msg.Ns {
				got, ok := ns.(*dns.SOA)
				if !ok {
					t.Errorf("Test %d: Unexpected NS type. Want: SOA, got: %v", ti, reflect.TypeOf(got))
				}
				if got.String() != tc.wantNS[i] {
					t.Errorf("Test %d: Unexpected NS.\nWant: %v\nGot: %v", ti, tc.wantNS[i], got)
				}
			}
		}
	}
}
