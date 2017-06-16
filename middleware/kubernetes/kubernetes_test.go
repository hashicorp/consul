package kubernetes

import (
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/miekg/dns"
	"k8s.io/client-go/1.5/pkg/api"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/etcd/msg"
	"github.com/coredns/coredns/request"
)

func TestRecordForTXT(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs.test"}}
	r, _ := k.parseRequest("dns-version.inter.webs.test", dns.TypeTXT)
	expected := DNSSchemaVersion

	var svcs []msg.Service
	k.recordsForTXT(r, &svcs)
	if svcs[0].Text != expected {
		t.Errorf("Expected result '%v'. Instead got result '%v'.", expected, svcs[0].Text)
	}
}

func TestPrimaryZone(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs.test", "inter.nets.test"}}
	expected := "inter.webs.test"
	result := k.PrimaryZone()
	if result != expected {
		t.Errorf("Expected result '%v'. Instead got result '%v'.", expected, result)
	}
}

func TestIsRequestInReverseRange(t *testing.T) {

	tests := []struct {
		cidr     string
		name     string
		expected bool
	}{
		{"1.2.3.0/24", "4.3.2.1.in-addr.arpa.", true},
		{"1.2.3.0/24", "5.3.2.1.in-addr.arpa.", true},
		{"1.2.3.0/24", "5.4.2.1.in-addr.arpa.", false},
		{"5.6.0.0/16", "5.4.2.1.in-addr.arpa.", false},
		{"5.6.0.0/16", "5.4.6.5.in-addr.arpa.", true},
		{"5.6.0.0/16", "5.6.0.1.in-addr.arpa.", false},
	}

	k := Kubernetes{Zones: []string{"inter.webs.test"}}

	for _, test := range tests {
		_, cidr, _ := net.ParseCIDR(test.cidr)
		k.ReverseCidrs = []net.IPNet{*cidr}
		result := k.isRequestInReverseRange(test.name)
		if result != test.expected {
			t.Errorf("Expected '%v' for '%v' in %v.", test.expected, test.name, test.cidr)
		}
	}
}

func TestIsNameError(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs.test"}}
	if !k.IsNameError(errNoItems) {
		t.Errorf("Expected 'true' for '%v'", errNoItems)
	}
	if !k.IsNameError(errNsNotExposed) {
		t.Errorf("Expected 'true' for '%v'", errNsNotExposed)
	}
	if !k.IsNameError(errInvalidRequest) {
		t.Errorf("Expected 'true' for '%v'", errInvalidRequest)
	}
	otherErr := errors.New("Some other error occured")
	if k.IsNameError(otherErr) {
		t.Errorf("Expected 'true' for '%v'", otherErr)
	}
}

func TestSymbolContainsWildcard(t *testing.T) {
	var testdataSymbolContainsWildcard = []struct {
		Symbol         string
		ExpectedResult bool
	}{
		{"mynamespace", false},
		{"*", true},
		{"any", true},
		{"my*space", false},
		{"*space", false},
		{"myname*", false},
	}

	for _, example := range testdataSymbolContainsWildcard {
		actualResult := symbolContainsWildcard(example.Symbol)
		if actualResult != example.ExpectedResult {
			t.Errorf("Expected SymbolContainsWildcard result '%v' for example string='%v'. Instead got result '%v'.", example.ExpectedResult, example.Symbol, actualResult)
		}
	}
}

func expectString(t *testing.T, function, qtype, query string, r *recordRequest, field, expected string) {
	ref := reflect.ValueOf(r)
	refField := reflect.Indirect(ref).FieldByName(field)
	got := refField.String()
	if got != expected {
		t.Errorf("Expected %v(%v, \"%v\") to get %v == \"%v\". Instead got \"%v\".", function, query, qtype, field, expected, got)
	}
}

func TestParseRequest(t *testing.T) {

	var tcs map[string]string

	k := Kubernetes{Zones: []string{"inter.webs.test"}}
	f := "parseRequest"

	// Test a valid SRV request
	//
	query := "_http._tcp.webs.mynamespace.svc.inter.webs.test."
	r, e := k.parseRequest(query, dns.TypeSRV)
	if e != nil {
		t.Errorf("Expected no error from parseRequest(%v, \"SRV\"). Instead got '%v'.", query, e)
	}

	tcs = map[string]string{
		"port":      "http",
		"protocol":  "tcp",
		"endpoint":  "",
		"service":   "webs",
		"namespace": "mynamespace",
		"typeName":  "svc",
		"zone":      "inter.webs.test",
	}
	for field, expected := range tcs {
		expectString(t, f, "SRV", query, &r, field, expected)
	}

	// Test wildcard acceptance
	//
	query = "*.any.*.any.svc.inter.webs.test."
	r, e = k.parseRequest(query, dns.TypeSRV)
	if e != nil {
		t.Errorf("Expected no error from parseRequest(\"%v\", \"SRV\"). Instead got '%v'.", query, e)
	}

	tcs = map[string]string{
		"port":      "*",
		"protocol":  "any",
		"endpoint":  "",
		"service":   "*",
		"namespace": "any",
		"typeName":  "svc",
		"zone":      "inter.webs.test",
	}
	for field, expected := range tcs {
		expectString(t, f, "SRV", query, &r, field, expected)
	}

	// Test A request of endpoint
	query = "1-2-3-4.webs.mynamespace.svc.inter.webs.test."
	r, e = k.parseRequest(query, dns.TypeA)
	if e != nil {
		t.Errorf("Expected no error from parseRequest(\"%v\", \"A\"). Instead got '%v'.", query, e)
	}
	tcs = map[string]string{
		"port":      "",
		"protocol":  "",
		"endpoint":  "1-2-3-4",
		"service":   "webs",
		"namespace": "mynamespace",
		"typeName":  "svc",
		"zone":      "inter.webs.test",
	}
	for field, expected := range tcs {
		expectString(t, f, "A", query, &r, field, expected)
	}

	// Test NS request
	query = "inter.webs.test."
	r, e = k.parseRequest(query, dns.TypeNS)
	if e != nil {
		t.Errorf("Expected no error from parseRequest(\"%v\", \"NS\"). Instead got '%v'.", query, e)
	}
	tcs = map[string]string{
		"port":      "",
		"protocol":  "",
		"endpoint":  "",
		"service":   "",
		"namespace": "",
		"typeName":  "",
		"zone":      "inter.webs.test",
	}
	for field, expected := range tcs {
		expectString(t, f, "NS", query, &r, field, expected)
	}

	// Test TXT request
	query = "dns-version.inter.webs.test."
	r, e = k.parseRequest(query, dns.TypeTXT)
	if e != nil {
		t.Errorf("Expected no error from parseRequest(\"%v\", \"TXT\"). Instead got '%v'.", query, e)
	}
	tcs = map[string]string{
		"port":      "",
		"protocol":  "",
		"endpoint":  "",
		"service":   "",
		"namespace": "",
		"typeName":  "dns-version",
		"zone":      "inter.webs.test",
	}
	for field, expected := range tcs {
		expectString(t, f, "TXT", query, &r, field, expected)
	}

	// Invalid query tests
	invalidAQueries := []string{
		"_http._tcp.webs.mynamespace.svc.inter.webs.test.", // A requests cannot have port or protocol
		"servname.ns1.srv.inter.nets.test.",                // A requests must have zone that matches corefile

	}
	for _, q := range invalidAQueries {
		_, e = k.parseRequest(q, dns.TypeA)
		if e == nil {
			t.Errorf("Expected error from %v(\"%v\", \"A\").", f, q)
		}
	}

	invalidSRVQueries := []string{
		"webs.mynamespace.svc.inter.webs.test.",            // SRV requests must have port and protocol
		"_http._pcp.webs.mynamespace.svc.inter.webs.test.", // SRV protocol must be tcp or udp
		"_http._tcp.ep.webs.ns.svc.inter.webs.test.",       // SRV requests cannot have an endpoint
		"_*._*.webs.mynamespace.svc.inter.webs.test.",      // SRV request with invalid wildcards
		"_http._tcp",
		"_tcp.test.",
		".",
	}

	for _, q := range invalidSRVQueries {
		_, e = k.parseRequest(q, dns.TypeSRV)
		if e == nil {
			t.Errorf("Expected error from %v(\"%v\", \"SRV\").", f, q)
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

func TestIpFromPodName(t *testing.T) {
	var tests = []struct {
		ip       string
		expected string
	}{
		{"10-11-12-13", "10.11.12.13"},
		{"1-2-3-4", "1.2.3.4"},
		{"1-2-3--A-B-C", "1:2:3::A:B:C"},
	}
	for _, test := range tests {
		result := ipFromPodName(test.ip)
		if result != test.expected {
			t.Errorf("Expected ip for podname '%v' to be '%v', but got '%v'", test.ip, test.expected, result)
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

	k := Kubernetes{Zones: []string{"interwebs.test"}}
	k.Federations = []Federation{{name: "fed", zone: "era.tion.com"}}
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
			Req: &dns.Msg{Question: []dns.Question{{Name: test.qname, Qtype: test.qtype}}},
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
