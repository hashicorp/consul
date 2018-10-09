package kubernetes

import (
	"strconv"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/kubernetes/object"
)

func endpointSubsets(addrs ...string) (eps []object.EndpointSubset) {
	for _, ap := range addrs {
		apa := strings.Split(ap, ":")
		address := apa[0]
		port, _ := strconv.Atoi(apa[1])
		eps = append(eps, object.EndpointSubset{Addresses: []object.EndpointAddress{{IP: address}}, Ports: []object.EndpointPort{{Port: int32(port)}}})
	}
	return eps
}

func TestEndpointsSubsetDiffs(t *testing.T) {
	var tests = []struct {
		a, b, expected object.Endpoints
	}{
		{ // From a->b: Nothing changes
			object.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			object.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			object.Endpoints{},
		},
		{ // From a->b: Everything goes away
			object.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			object.Endpoints{},
			object.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
		},
		{ // From a->b: Everything is new
			object.Endpoints{},
			object.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
			object.Endpoints{Subsets: endpointSubsets("10.0.0.1:80", "10.0.0.2:8080")},
		},
		{ // From a->b: One goes away, one is new
			object.Endpoints{Subsets: endpointSubsets("10.0.0.2:8080")},
			object.Endpoints{Subsets: endpointSubsets("10.0.0.1:80")},
			object.Endpoints{Subsets: endpointSubsets("10.0.0.2:8080", "10.0.0.1:80")},
		},
	}

	for i, te := range tests {
		got := endpointsSubsetDiffs(&te.a, &te.b)
		if !endpointsEquivalent(got, &te.expected) {
			t.Errorf("Expected '%v' for test %v, got '%v'.", te.expected, i, got)
		}
	}
}
