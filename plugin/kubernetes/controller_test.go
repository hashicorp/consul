package kubernetes

import (
	"strconv"
	"strings"
	"testing"

	api "k8s.io/api/core/v1"
)

func endpointSubsets(addrs ...string) (eps []api.EndpointSubset) {
	for _, ap := range addrs {
		apa := strings.Split(ap, ":")
		address := apa[0]
		port, _ := strconv.Atoi(apa[1])
		eps = append(eps, api.EndpointSubset{Addresses: []api.EndpointAddress{{IP: address}}, Ports: []api.EndpointPort{{Port: int32(port)}}})
	}
	return eps
}

func TestEndpointsSubsetDiffs(t *testing.T) {
	var tests = []struct {
		a, b, expected api.Endpoints
	}{
		{ // From a->b: Nothing changes
			api.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			api.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			api.Endpoints{},
		},
		{ // From a->b: Everything goes away
			api.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			api.Endpoints{},
			api.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
		},
		{ // From a->b: Everything is new
			api.Endpoints{},
			api.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			api.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
		},
		{ // From a->b: One goes away, one is new
			api.Endpoints{Subsets: endpointSubsets("10.0.0.2:8080")},
			api.Endpoints{Subsets: endpointSubsets("10.0.0.1:80")},
			api.Endpoints{Subsets: endpointSubsets("10.0.0.2:8080", "10.0.0.1:80")},
		},
	}

	for i, te := range tests {
		got := endpointsSubsetDiffs(&te.a, &te.b)
		if !endpointsEquivalent(got, &te.expected) {
			t.Errorf("Expected '%v' for test %v, got '%v'.", te.expected, i, got)
		}
	}
}
