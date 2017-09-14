package msg

import (
	"testing"

	"github.com/miekg/dns"
)

func TestType(t *testing.T) {
	tests := []struct {
		serv         Service
		expectedType uint16
	}{
		{Service{Host: "example.org"}, dns.TypeCNAME},
		{Service{Host: "127.0.0.1"}, dns.TypeA},
		{Service{Host: "2000::3"}, dns.TypeAAAA},
		{Service{Host: "2000..3"}, dns.TypeCNAME},
		{Service{Host: "127.0.0.257"}, dns.TypeCNAME},
		{Service{Host: "127.0.0.252", Mail: true}, dns.TypeA},
		{Service{Host: "127.0.0.252", Mail: true, Text: "a"}, dns.TypeA},
		{Service{Host: "127.0.0.254", Mail: false, Text: "a"}, dns.TypeA},
	}

	for i, tc := range tests {
		what, _ := tc.serv.HostType()
		if what != tc.expectedType {
			t.Errorf("Test %d: Expected what %v, but got %v", i, tc.expectedType, what)
		}
	}

}
