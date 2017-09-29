package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	api "k8s.io/client-go/pkg/api/v1"
)

func TestWildcard(t *testing.T) {
	var tests = []struct {
		s        string
		expected bool
	}{
		{"mynamespace", false},
		{"*", true},
		{"any", true},
		{"my*space", false},
		{"*space", false},
		{"myname*", false},
	}

	for _, te := range tests {
		got := wildcard(te.s)
		if got != te.expected {
			t.Errorf("Expected Wildcard result '%v' for example '%v', got '%v'.", te.expected, te.s, got)
		}
	}
}

func TestEndpointHostname(t *testing.T) {
	var tests = []struct {
		ip       string
		hostname string
		expected string
	}{
		{"10.11.12.13", "", "10-11-12-13"},
		{"10.11.12.13", "epname", "epname"},
	}
	for _, test := range tests {
		result := endpointHostname(api.EndpointAddress{IP: test.ip, Hostname: test.hostname})
		if result != test.expected {
			t.Errorf("Expected endpoint name for (ip:%v hostname:%v) to be '%v', but got '%v'", test.ip, test.hostname, test.expected, result)
		}
	}
}

type APIConnServiceTest struct{}

func (APIConnServiceTest) Run()                       { return }
func (APIConnServiceTest) Stop() error                { return nil }
func (APIConnServiceTest) PodIndex(string) []*api.Pod { return nil }

func (APIConnServiceTest) ServiceList() []*api.Service {
	svcs := []*api.Service{
		{
			ObjectMeta: meta.ObjectMeta{
				Name:      "svc1",
				Namespace: "testns",
			},
			Spec: api.ServiceSpec{
				ClusterIP: "10.0.0.1",
				Ports: []api.ServicePort{{
					Name:     "http",
					Protocol: "tcp",
					Port:     80,
				}},
			},
		},
		{
			ObjectMeta: meta.ObjectMeta{
				Name:      "hdls1",
				Namespace: "testns",
			},
			Spec: api.ServiceSpec{
				ClusterIP: api.ClusterIPNone,
			},
		},
		{
			ObjectMeta: meta.ObjectMeta{
				Name:      "external",
				Namespace: "testns",
			},
			Spec: api.ServiceSpec{
				ExternalName: "coredns.io",
				Ports: []api.ServicePort{{
					Name:     "http",
					Protocol: "tcp",
					Port:     80,
				}},
			},
		},
	}
	return svcs
}

func (APIConnServiceTest) EndpointsList() []*api.Endpoints {
	n := "test.node.foo.bar"

	eps := []*api.Endpoints{
		{
			Subsets: []api.EndpointSubset{
				{
					Addresses: []api.EndpointAddress{
						{
							IP:       "172.0.0.1",
							Hostname: "ep1a",
						},
					},
					Ports: []api.EndpointPort{
						{
							Port:     80,
							Protocol: "tcp",
							Name:     "http",
						},
					},
				},
			},
			ObjectMeta: meta.ObjectMeta{
				Name:      "svc1",
				Namespace: "testns",
			},
		},
		{
			Subsets: []api.EndpointSubset{
				{
					Addresses: []api.EndpointAddress{
						{
							IP: "172.0.0.2",
						},
					},
					Ports: []api.EndpointPort{
						{
							Port:     80,
							Protocol: "tcp",
							Name:     "http",
						},
					},
				},
			},
			ObjectMeta: meta.ObjectMeta{
				Name:      "hdls1",
				Namespace: "testns",
			},
		},
		{
			Subsets: []api.EndpointSubset{
				{
					Addresses: []api.EndpointAddress{
						{
							IP: "172.0.0.3",
						},
					},
					Ports: []api.EndpointPort{
						{
							Port:     80,
							Protocol: "tcp",
							Name:     "http",
						},
					},
				},
			},
			ObjectMeta: meta.ObjectMeta{
				Name:      "hdls1",
				Namespace: "testns",
			},
		},
		{
			Subsets: []api.EndpointSubset{
				{
					Addresses: []api.EndpointAddress{
						{
							IP:       "10.9.8.7",
							NodeName: &n,
						},
					},
				},
			},
		},
	}
	return eps
}

func (APIConnServiceTest) GetNodeByName(name string) (*api.Node, error) {
	return &api.Node{
		ObjectMeta: meta.ObjectMeta{
			Name: "test.node.foo.bar",
		},
	}, nil
}

func TestServices(t *testing.T) {

	k := New([]string{"interwebs.test."})
	k.APIConn = &APIConnServiceTest{}

	type svcAns struct {
		host string
		key  string
	}
	type svcTest struct {
		qname  string
		qtype  uint16
		answer svcAns
	}
	tests := []svcTest{
		// Cluster IP Services
		{qname: "svc1.testns.svc.interwebs.test.", qtype: dns.TypeA, answer: svcAns{host: "10.0.0.1", key: "/coredns/test/interwebs/svc/testns/svc1"}},
		{qname: "_http._tcp.svc1.testns.svc.interwebs.test.", qtype: dns.TypeSRV, answer: svcAns{host: "10.0.0.1", key: "/coredns/test/interwebs/svc/testns/svc1"}},
		{qname: "ep1a.svc1.testns.svc.interwebs.test.", qtype: dns.TypeA, answer: svcAns{host: "172.0.0.1", key: "/coredns/test/interwebs/svc/testns/svc1/ep1a"}},

		// External Services
		{qname: "external.testns.svc.interwebs.test.", qtype: dns.TypeCNAME, answer: svcAns{host: "coredns.io", key: "/coredns/test/interwebs/svc/testns/external"}},
	}

	for i, test := range tests {
		state := request.Request{
			Req:  &dns.Msg{Question: []dns.Question{{Name: test.qname, Qtype: test.qtype}}},
			Zone: "interwebs.test.", // must match from k.Zones[0]
		}
		svcs, e := k.Services(state, false, plugin.Options{})
		if e != nil {
			t.Errorf("Test %d: got error '%v'", i, e)
			continue
		}
		if len(svcs) != 1 {
			t.Errorf("Test %d, expected expected 1 answer, got %v", i, len(svcs))
			continue
		}

		if test.answer.host != svcs[0].Host {
			t.Errorf("Test %d, expected host '%v', got '%v'", i, test.answer.host, svcs[0].Host)
		}
		if test.answer.key != svcs[0].Key {
			t.Errorf("Test %d, expected key '%v', got '%v'", i, test.answer.key, svcs[0].Key)
		}
	}
}
