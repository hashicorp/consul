package test

import (
	"testing"

	"github.com/miekg/coredns/core"
)

// Bind to low port should fail.
func TestFailStartServer(t *testing.T) {
	corefile := `.:53 {
	chaos CoreDNS-001 miek@miek.nl
}
`
	srv, _ := core.TestServer(t, corefile)
	err := srv.ListenAndServe()
	if err == nil {
		srv.Stop()
		t.Fatalf("Low port startup should fail")
	}
}
