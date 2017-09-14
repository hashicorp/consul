package dnsutil

import (
	"testing"

	"github.com/miekg/dns"
)

func TestDuplicateCNAME(t *testing.T) {
	tests := []struct {
		cname    string
		records  []string
		expected bool
	}{
		{
			"1.0.0.192.IN-ADDR.ARPA.	3600	IN	CNAME	1.0.0.0.192.IN-ADDR.ARPA.",
			[]string{
				"US.    86400	IN	NSEC	0-.us. NS SOA RRSIG NSEC DNSKEY TYPE65534",
				"1.0.0.192.IN-ADDR.ARPA.	3600	IN	CNAME	1.0.0.0.192.IN-ADDR.ARPA.",
			},
			true,
		},
		{
			"1.0.0.192.IN-ADDR.ARPA.	3600	IN	CNAME	1.0.0.0.192.IN-ADDR.ARPA.",
			[]string{
				"US.    86400	IN	NSEC	0-.us. NS SOA RRSIG NSEC DNSKEY TYPE65534",
			},
			false,
		},
		{
			"1.0.0.192.IN-ADDR.ARPA.	3600	IN	CNAME	1.0.0.0.192.IN-ADDR.ARPA.",
			[]string{},
			false,
		},
	}
	for i, test := range tests {
		cnameRR, err := dns.NewRR(test.cname)
		if err != nil {
			t.Fatalf("Test %d, cname ('%s') error (%s)!", i, test.cname, err)
		}
		cname := cnameRR.(*dns.CNAME)
		records := []dns.RR{}
		for j, r := range test.records {
			rr, err := dns.NewRR(r)
			if err != nil {
				t.Fatalf("Test %d, record %d ('%s') error (%s)!", i, j, r, err)
			}
			records = append(records, rr)
		}
		got := DuplicateCNAME(cname, records)
		if got != test.expected {
			t.Errorf("Test %d, expected '%v', got '%v' for CNAME ('%s') and RECORDS (%v)", i, test.expected, got, test.cname, test.records)
		}
	}
}
