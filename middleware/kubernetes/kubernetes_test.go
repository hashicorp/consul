package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"k8s.io/client-go/1.5/pkg/api"
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

func (APIConnServiceTest) Run()                          { return }
func (APIConnServiceTest) Stop() error                   { return nil }
func (APIConnServiceTest) PodIndex(string) []interface{} { return nil }

func (APIConnServiceTest) ServiceList() []*api.Service {
	svcs := []*api.Service{
		{
			ObjectMeta: api.ObjectMeta{
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
			ObjectMeta: api.ObjectMeta{
				Name:      "hdls1",
				Namespace: "testns",
			},
			Spec: api.ServiceSpec{
				ClusterIP: api.ClusterIPNone,
			},
		},
		{
			ObjectMeta: api.ObjectMeta{
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

func (APIConnServiceTest) EndpointsList() api.EndpointsList {
	n := "test.node.foo.bar"

	return api.EndpointsList{
		Items: []api.Endpoints{
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
				ObjectMeta: api.ObjectMeta{
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
				ObjectMeta: api.ObjectMeta{
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
				ObjectMeta: api.ObjectMeta{
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
		},
	}
}

func (APIConnServiceTest) GetNodeByName(name string) (api.Node, error) {
	return api.Node{
		ObjectMeta: api.ObjectMeta{
			Name: "test.node.foo.bar",
			Labels: map[string]string{
				labelRegion:           "fd-r",
				labelAvailabilityZone: "fd-az",
			},
		},
	}, nil
}

func TestServices(t *testing.T) {

	k := Kubernetes{Zones: []string{"interwebs.test."}}
	k.Federations = []Federation{{name: "fed", zone: "era.tion.com"}}
	k.interfaceAddrsFunc = localPodIP
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

		// Federated Services
		{qname: "svc1.testns.fed.svc.interwebs.test.", qtype: dns.TypeA, answer: svcAns{host: "10.0.0.1", key: "/coredns/test/interwebs/svc/fed/testns/svc1"}},
		{qname: "svc0.testns.fed.svc.interwebs.test.", qtype: dns.TypeA, answer: svcAns{host: "svc0.testns.fed.svc.fd-az.fd-r.era.tion.com", key: "/coredns/test/interwebs/svc/fed/testns/svc0"}},
	}

	for _, test := range tests {
		state := request.Request{
			Req:  &dns.Msg{Question: []dns.Question{{Name: test.qname, Qtype: test.qtype}}},
			Zone: "interwebs.test.", // must match from k.Zones[0]
		}
		svcs, _, e := k.Services(state, false, middleware.Options{})
		if e != nil {
			t.Errorf("Query '%v' got error '%v'", test.qname, e)
		}
		if len(svcs) != 1 {
			t.Errorf("Query %v %v: expected expected 1 answer, got %v", test.qname, dns.TypeToString[test.qtype], len(svcs))
		} else {
			if test.answer.host != svcs[0].Host {
				t.Errorf("Query %v %v: expected host '%v', got '%v'", test.qname, dns.TypeToString[test.qtype], test.answer.host, svcs[0].Host)
			}
			if test.answer.key != svcs[0].Key {
				t.Errorf("Query %v %v: expected key '%v', got '%v'", test.qname, dns.TypeToString[test.qtype], test.answer.key, svcs[0].Key)
			}
		}
	}
}
