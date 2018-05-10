package connect

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestSpiffeIDService returns a SPIFFE ID representing a service.
func TestSpiffeIDService(t testing.T, service string) *SpiffeIDService {
	return TestSpiffeIDServiceWithHost(t, service, TestClusterID+".consul")
}

// TestSpiffeIDServiceWithHost returns a SPIFFE ID representing a service with
// the specified trust domain.
func TestSpiffeIDServiceWithHost(t testing.T, service, host string) *SpiffeIDService {
	return &SpiffeIDService{
		Host:       host,
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    service,
	}
}
