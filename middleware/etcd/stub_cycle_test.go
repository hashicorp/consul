// +build etcd

package etcd

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/dns"
)

func TestStubCycle(t *testing.T) {
	// reuse servics from stub_test.go
	for _, serv := range servicesStub {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}
	etc.updateStubZones()

	for _, tc := range dnsTestCasesCycleStub {
		m := tc.Msg()
		if tc.Do {
			// add our wacky edns fluff
			m.Extra[0] = ednsStub
		}

		rec := middleware.NewResponseRecorder(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err == nil {
			t.Errorf("expected error, got none")
			continue
		}
		// err should have been, set msg is nil, CoreDNS middlware handling takes
		// care of proper error to client.
	}
}

var dnsTestCasesCycleStub = []test.Case{{Qname: "example.org.", Qtype: dns.TypeA, Rcode: dns.RcodeRefused, Do: true}}
