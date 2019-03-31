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
	rrsResponse := map[string][]*route53.ResourceRecordSet{}
	for _, r := range []struct {
		rType, name, value, hostedZoneID string
	}{
		{"A", "example.org.", "1.2.3.4", "1234567890"},
		{"A", "www.example.org", "1.2.3.4", "1234567890"},
		{"CNAME", `\\052.www.example.org`, "www.example.org", "1234567890"},
		{"AAAA", "example.org.", "2001:db8:85a3::8a2e:370:7334", "1234567890"},
		{"CNAME", "sample.example.org.", "example.org", "1234567890"},
		{"PTR", "example.org.", "ptr.example.org.", "1234567890"},
		{"SOA", "org.", "ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400", "1234567890"},
		{"NS", "com.", "ns-1536.awsdns-00.co.uk.", "1234567890"},
		{"A", "split-example.gov.", "1.2.3.4", "1234567890"},
		// Unsupported type should be ignored.
		{"YOLO", "swag.", "foobar", "1234567890"},
		// Hosted zone with the same name, but a different id.
		{"A", "other-example.org.", "3.5.7.9", "1357986420"},
		{"A", "split-example.org.", "1.2.3.4", "1357986420"},
		{"SOA", "org.", "ns-15.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400", "1357986420"},
		// Hosted zone without SOA.
	} {
		rrs, ok := rrsResponse[r.hostedZoneID]
		if !ok {
			rrs = make([]*route53.ResourceRecordSet, 0)
		}
		rrs = append(rrs, &route53.ResourceRecordSet{Type: aws.String(r.rType),
			Name: aws.String(r.name),
			ResourceRecords: []*route53.ResourceRecord{
				{
					Value: aws.String(r.value),
				},
			},
			TTL: aws.Int64(300),
		})
		rrsResponse[r.hostedZoneID] = rrs
	}

	if ok := fn(&route53.ListResourceRecordSetsOutput{
		ResourceRecordSets: rrsResponse[aws.StringValue(in.HostedZoneId)],
	}, true); !ok {
		return errors.New("paging function return false")
	}
	return nil
}

func TestRoute53(t *testing.T) {
	ctx := context.Background()

	r, err := New(ctx, fakeRoute53{}, map[string][]string{"bad.": []string{"0987654321"}}, &upstream.Upstream{})
	if err != nil {
		t.Fatalf("Failed to create Route53: %v", err)
	}
	if err = r.Run(ctx); err == nil {
		t.Fatalf("Expected errors for zone bad.")
	}

	r, err = New(ctx, fakeRoute53{}, map[string][]string{"org.": []string{"1357986420", "1234567890"}, "gov.": []string{"Z098765432", "1234567890"}}, &upstream.Upstream{})
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

			m.Authoritative = true
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
			wantAnswer: []string{"org.	300	IN	SOA	ns-15.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 6. Explicit SOA query for example.org.
		{
			qname: "example.org",
			qtype: dns.TypeNS,
			wantNS: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
		},
		// 7. AAAA query for split-example.org must return NODATA.
		{
			qname:       "split-example.gov",
			qtype:       dns.TypeAAAA,
			wantRetCode: dns.RcodeSuccess,
			wantNS: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
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
			wantNS: []string{"org.	300	IN	SOA	ns-1536.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
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
			wantNS: []string{"org.	300	IN	SOA	ns-15.awsdns-00.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"},
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

func TestMaybeUnescape(t *testing.T) {
	for ti, tc := range []struct {
		escaped, want string
		wantErr       error
	}{
		// 0. empty string is fine.
		{escaped: "", want: ""},
		// 1. non-escaped sequence.
		{escaped: "example.com.", want: "example.com."},
		// 2. escaped `*` as first label - OK.
		{escaped: `\\052.example.com`, want: "*.example.com"},
		// 3. Escaped dot, 'a' and a hyphen. No idea why but we'll allow it.
		{escaped: `weird\\055ex\\141mple\\056com\\056\\056`, want: "weird-example.com.."},
		// 4. escaped `*` in the middle - NOT OK.
		{escaped: `e\\052ample.com`, wantErr: errors.New("`*' ony supported as wildcard (leftmost label)")},
		// 5. Invalid character.
		{escaped: `\\000.example.com`, wantErr: errors.New(`invalid character: \\000`)},
		// 6. Invalid escape sequence in the middle.
		{escaped: `example\\0com`, wantErr: errors.New(`invalid escape sequence: '\\0co'`)},
		// 7. Invalid escape sequence at the end.
		{escaped: `example.com\\0`, wantErr: errors.New(`invalid escape sequence: '\\0'`)},
	} {
		got, gotErr := maybeUnescape(tc.escaped)
		if tc.wantErr != gotErr && !reflect.DeepEqual(tc.wantErr, gotErr) {
			t.Fatalf("Test %d: Expected error: `%v', but got: `%v'", ti, tc.wantErr, gotErr)
		}
		if tc.want != got {
			t.Errorf("Test %d: Expected unescaped: `%s', but got: `%s'", ti, tc.want, got)
		}
	}
}
