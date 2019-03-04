package test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/miekg/dns"
)

func setupProxyTargetCoreDNS(t *testing.T, fn func(string)) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "coredns")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpdir)

	content := `
example.org. IN	SOA sns.dns.icann.org. noc.dns.icann.org. 1 3600 3600 3600 3600

google.com. IN SOA ns1.google.com. dns-admin.google.com. 1 3600 3600 3600 3600
google.com. IN A 172.217.25.110
`

	path := filepath.Join(tmpdir, "file")
	if err = ioutil.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Could not write to temp file: %s", err)
	}
	defer os.Remove(path)

	corefile := `.:0 {
	file ` + path + `
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get proxy target CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	fn(udp)
}

func TestLookupAutoPathErratic(t *testing.T) {
	setupProxyTargetCoreDNS(t, func(proxyPath string) {
		corefile := `.:0 {
		erratic
		autopath @erratic
		forward . ` + proxyPath + `
		debug
		}
`
		i, udp, _, err := CoreDNSServerAndPorts(corefile)
		if err != nil {
			t.Fatalf("Could not get CoreDNS serving instance: %s", err)
		}
		defer i.Stop()

		tests := []struct {
			qname          string
			expectedAnswer string
			expectedType   uint16
		}{
			{"google.com.a.example.org.", "google.com.a.example.org.", dns.TypeCNAME},
			{"google.com.", "google.com.", dns.TypeA},
		}

		for i, tc := range tests {
			m := new(dns.Msg)
			// erratic always returns this search path: "a.example.org.", "b.example.org.", "".
			m.SetQuestion(tc.qname, dns.TypeA)

			r, err := dns.Exchange(m, udp)
			if err != nil {
				t.Fatalf("Test %d, failed to sent query: %q", i, err)
			}
			if len(r.Answer) == 0 {
				t.Fatalf("Test %d, answer section should have RRs", i)
			}
			if x := r.Answer[0].Header().Name; x != tc.expectedAnswer {
				t.Fatalf("Test %d, expected answer %s, got %s", i, tc.expectedAnswer, x)
			}
			if x := r.Answer[0].Header().Rrtype; x != tc.expectedType {
				t.Fatalf("Test %d, expected answer type %d, got %d", i, tc.expectedType, x)
			}
		}
	})
}

func TestAutoPathErraticNotLoaded(t *testing.T) {
	setupProxyTargetCoreDNS(t, func(proxyPath string) {
		corefile := `.:0 {
	autopath @erratic
	forward . ` + proxyPath + `
	debug
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

		m := new(dns.Msg)
		m.SetQuestion("google.com.a.example.org.", dns.TypeA)
		r, err := dns.Exchange(m, udp)
		if err != nil {
			t.Fatalf("Failed to sent query: %q", err)
		}
		if r.Rcode != dns.RcodeNameError {
			t.Fatalf("Expected NXDOMAIN, got %d", r.Rcode)
		}
	})
}
