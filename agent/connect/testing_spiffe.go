package connect

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestSpiffeIDConsulService returns a SPIFFE ID representing a service.
func TestSpiffeIDConsulService(t testing.T, service string) *SpiffeIDConsulService {
	return TestSpiffeIDConsulServiceWithHost(t, service, TestClusterID+".consul")
}

// TestSpiffeIDConsulServiceWithHost returns a SPIFFE ID representing a service with
// the specified trust domain.
func TestSpiffeIDConsulServiceWithHost(t testing.T, service, host string) *SpiffeIDConsulService {
	return &SpiffeIDConsulService{
		Host:       host,
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    service,
	}
}
