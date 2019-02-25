package errors

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func BenchmarkServeDNS(b *testing.B) {
	h := &errorHandler{}
	h.Next = test.ErrorHandler()

	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)
	w := &test.ResponseWriter{}
	ctx := context.TODO()

	for i := 0; i < b.N; i++ {
		_, err := h.ServeDNS(ctx, w, r)
		if err != nil {
			b.Errorf("ServeDNS returned error: %s", err)
		}
	}
}
