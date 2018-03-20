package connect

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestSpiffeIDService returns a SPIFFE ID representing a service.
func TestSpiffeIDService(t testing.T, service string) *SpiffeIDService {
	return &SpiffeIDService{
		Host:       testClusterID + ".consul",
		Namespace:  "default",
		Datacenter: "dc01",
		Service:    service,
	}
}
