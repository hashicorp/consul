package route53

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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/miekg/dns"
)

type fakeRoute53 struct {
	route53iface.Route53API
}

func (fakeRoute53) ListHostedZonesByNameWithContext(_ aws.Context, input *route53.ListHostedZonesByNameInput, _ ...request.Option) (*route53.ListHostedZonesByNameOutput, error) {
	return nil, nil
}

func (fakeRoute53) ListResourceRecordSetsPagesWithContext(_ aws.Context, in *route53.ListResourceRecordSetsInput, fn func(*route53.ListResourceRecordSetsOutput, bool) bool, _ ...request.Option) error {
	if aws.StringValue(in.HostedZoneId) == "0987654321" {
		return errors.New("bad. zone is bad")
	}
	var rrs []*route53.ResourceRecordSet
	for _, r := range []struct {
		rType, name, value string
	}{
		{"A", "example.org.", "1.2.3.4"},
		{"AAAA", "example.org.", "2001:db8:85a3::8a2e:370:7334"},
		{"CNAME", "sample.example.org.", "example.org"},
		{"PTR", "example.org.", "ptr.example.org."},
		{"SOA", "org.", "ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		{"NS", "com.", "ns-1536.awsdns-00.co.uk."},
		// Unsupported type should be ignored.
		{"YOLO", "swag.", "foobar"},
	} {
		rrs = append(rrs, &route53.ResourceRecordSet{Type: aws.String(r.rType),
			Name: aws.String(r.name),
			ResourceRecords: []*route53.ResourceRecord{
				{
					Value: aws.String(r.value),
				},
			},
			TTL: aws.Int64(300),
		})
	}
	if ok := fn(&route53.ListResourceRecordSetsOutput{
		ResourceRecordSets: rrs,
	}, true); !ok {
		return errors.New("paging function return false")
	}
	return nil
}

func TestRoute53(t *testing.T) {
	ctx := context.Background()

	r, err := New(ctx, fakeRoute53{}, map[string]string{"bad.": "0987654321"}, &upstream.Upstream{})
	if err != nil {
		t.Fatalf("Failed to create Route53: %v", err)
	}
	if err = r.Run(ctx); err == nil {
		t.Fatalf("Expected errors for zone bad.")
	}

	r, err = New(ctx, fakeRoute53{}, map[string]string{"org.": "1234567890", "gov.": "Z098765432"}, &upstream.Upstream{})
	if err != nil {
		t.Fatalf("Failed to create Route53: %v", err)
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

			m.Authoritative, m.RecursionAvailable = true, true
			rcode = dns.RcodeSuccess
		}

		m.SetRcode(r, rcode)
		w.WriteMsg(m)
		return rcode, nil
	})
	err = r.Run(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize Route53: %v", err)
	}

	tests := []struct {
		qname        string
		qtype        uint16
		expectedCode int
		wantAnswer   []string // ownernames for the records in the additional section.
		wantNS       []string
		expectedErr  error
	}{
		// 0. example.org A found - success.
		{
			qname:        "example.org",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.org.	300	IN	A	1.2.3.4"},
		},
		// 1. example.org AAAA found - success.
		{
			qname:        "example.org",
			qtype:        dns.TypeAAAA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.org.	300	IN	AAAA	2001:db8:85a3::8a2e:370:7334"},
		},
		// 2. exampled.org PTR found - success.
		{
			qname:        "example.org",
			qtype:        dns.TypePTR,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.org.	300	IN	PTR	ptr.example.org."},
		},
		// 3. sample.example.org points to example.org CNAME.
		// Query must return both CNAME and A recs.
		{
			qname:        "sample.example.org",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{
				"sample.example.org.	300	IN	CNAME	example.org.",
				"example.org.	300	IN	A	1.2.3.4",
			},
		},
		// 4. Explicit CNAME query for sample.example.org.
		// Query must return just CNAME.
		{
			qname:        "sample.example.org",
			qtype:        dns.TypeCNAME,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"sample.example.org.	300	IN	CNAME	example.org."},
		},
		// 5. Explicit SOA query for example.org.
		{
			qname:        "example.org",
			qtype:        dns.TypeSOA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 6. Explicit SOA query for example.org.
		{
			qname:        "example.org",
			qtype:        dns.TypeNS,
			expectedCode: dns.RcodeSuccess,
			wantNS: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 7. Zone not configured.
		{
			qname:        "badexample.com",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeServerFailure,
		},
		// 8. No record found. Return SOA record.
		{
			qname:        "bad.org",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantNS: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 9. No record found. Fallthrough.
		{
			qname:        "example.gov",
			qtype:        dns.TypeA,
			expectedCode: dns.RcodeSuccess,
			wantAnswer: []string{"example.gov.	300	IN	A	2.4.6.8"},
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
		if code != int(tc.expectedCode) {
			t.Fatalf("Test %d: Expected status code %s, but got %s", ti, dns.RcodeToString[tc.expectedCode], dns.RcodeToString[code])
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
					t.Errorf("Test %d: Unexpected NS. Want: %v, got: %v", ti, tc.wantNS[i], got)
				}
			}
		}
	}
}
