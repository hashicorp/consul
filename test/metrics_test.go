package test

import "testing"

// Start test server that has metrics enabled. Then tear it down again.
func TestMetricsServer(t *testing.T) {
	corefile := `.:0 {
	chaos CoreDNS-001 miek@miek.nl
	prometheus localhost:0
}
`
	srv, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer srv.Stop()
}
