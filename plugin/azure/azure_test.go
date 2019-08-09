package azure

import (
	"context"
	"reflect"
	"testing"

	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

var demoAzure = Azure{
	Next:      testHandler(),
	Fall:      fall.Zero,
	zoneNames: []string{"example.org.", "www.example.org.", "example.org.", "sample.example.org."},
	zones:     testZones(),
}

func testZones() zones {
	zones := make(map[string][]*zone)
	zones["example.org."] = append(zones["example.org."], &zone{zone: "example.org."})
	newZ := file.NewZone("example.org.", "")

	for _, rr := range []string{
		"example.org.  300 IN  A   1.2.3.4",
		"example.org.  300 IN  AAAA   2001:db8:85a3::8a2e:370:7334",
		"www.example.org.  300 IN  A   1.2.3.4",
		"www.example.org.  300 IN  A   1.2.3.4",
		"org.	172800	IN	NS	ns3-06.azure-dns.org.",
		"org.	300	IN	SOA	ns1-06.azure-dns.com. azuredns-hostmaster.microsoft.com. 1 3600 300 2419200 300",
		"cname.example.org. 300 IN CNAME example.org",
		"mail.example.org. 300 IN MX 10 mailserver.example.com",
		"ptr.example.org. 300 IN PTR www.ptr-example.com",
		"example.org. 300 IN SRV 1 10 5269 srv-1.example.com.",
		"example.org. 300 IN SRV 1 10 5269 srv-2.example.com.",
		"txt.example.org. 300 IN TXT \"TXT for example.org\"",
	} {
		r, _ := dns.NewRR(rr)
		newZ.Insert(r)
	}
	zones["example.org."][0].z = newZ
	return zones
}

func testHandler() test.HandlerFunc {
	return func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		state := request.Request{W: w, Req: r}
		qname := state.Name()
		m := new(dns.Msg)
		rcode := dns.RcodeServerFailure
		if qname == "example.gov." { // No records match, test fallthrough.
			m.SetReply(r)
			rr := test.A("example.gov.  300 IN  A   2.4.6.8")
			m.Answer = []dns.RR{rr}
			m.Authoritative = true
			rcode = dns.RcodeSuccess
		}
		m.SetRcode(r, rcode)
		w.WriteMsg(m)
		return rcode, nil
	}
}

func TestAzure(t *testing.T) {
	tests := []struct {
		qname        string
		qtype        uint16
		wantRetCode  int
		wantAnswer   []string
		wantMsgRCode int
		wantNS       []string
		expectedErr  error
	}{
		{
			qname: "example.org.",
			qtype: dns.TypeA,
			wantAnswer: []string{"example.org.	300	IN	A	1.2.3.4"},
		},
		{
			qname: "example.org",
			qtype: dns.TypeAAAA,
			wantAnswer: []string{"example.org.	300	IN	AAAA	2001:db8:85a3::8a2e:370:7334"},
		},
		{
			qname: "example.org",
			qtype: dns.TypeSOA,
			wantAnswer: []string{"org.	300	IN	SOA	ns1-06.azure-dns.com. azuredns-hostmaster.microsoft.com. 1 3600 300 2419200 300"},
		},
		{
			qname:        "badexample.com",
			qtype:        dns.TypeA,
			wantRetCode:  dns.RcodeServerFailure,
			wantMsgRCode: dns.RcodeServerFailure,
		},
		{
			qname: "example.gov",
			qtype: dns.TypeA,
			wantAnswer: []string{"example.gov.	300	IN	A	2.4.6.8"},
		},
		{
			qname: "example.org",
			qtype: dns.TypeSRV,
			wantAnswer: []string{"example.org.	300	IN	SRV	1 10 5269 srv-1.example.com.", "example.org.	300	IN	SRV	1 10 5269 srv-2.example.com."},
		},
		{
			qname: "cname.example.org.",
			qtype: dns.TypeCNAME,
			wantAnswer: []string{"cname.example.org.	300	IN	CNAME	example.org."},
		},
		{
			qname: "cname.example.org.",
			qtype: dns.TypeA,
			wantAnswer: []string{"cname.example.org.	300	IN	CNAME	example.org.", "example.org.	300	IN	A	1.2.3.4"},
		},
		{
			qname: "mail.example.org.",
			qtype: dns.TypeMX,
			wantAnswer: []string{"mail.example.org.	300	IN	MX	10 mailserver.example.com."},
		},
		{
			qname: "ptr.example.org.",
			qtype: dns.TypePTR,
			wantAnswer: []string{"ptr.example.org.	300	IN	PTR	www.ptr-example.com."},
		},
		{
			qname: "txt.example.org.",
			qtype: dns.TypeTXT,
			wantAnswer: []string{"txt.example.org.	300	IN	TXT	\"TXT for example.org\""},
		},
	}

	for ti, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := demoAzure.ServeDNS(context.Background(), rec, req)

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
