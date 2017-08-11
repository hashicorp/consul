package kubernetes

import (
	"net"
	"strings"
	"testing"

	"github.com/coredns/coredns/middleware/etcd/msg"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"k8s.io/client-go/1.5/pkg/api"
)

func testStripFederation(t *testing.T, k Kubernetes, input []string, expectedFed string, expectedSegs string) {
	fed, segs := k.stripFederation(input)

	if expectedSegs != strings.Join(segs, ".") {
		t.Errorf("For '%v', expected segs result '%v'. Instead got result '%v'.", strings.Join(input, "."), expectedSegs, strings.Join(segs, "."))
	}
	if expectedFed != fed {
		t.Errorf("For '%v', expected fed result '%v'. Instead got result '%v'.", strings.Join(input, "."), expectedFed, fed)
	}
}

func TestStripFederation(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs.test"}}
	k.Federations = []Federation{{name: "fed", zone: "era.tion.com"}}

	testStripFederation(t, k, []string{"service", "ns", "fed", Svc}, "fed", "service.ns.svc")
	testStripFederation(t, k, []string{"service", "ns", "foo", Svc}, "", "service.ns.foo.svc")
	testStripFederation(t, k, []string{"foo", "bar"}, "", "foo.bar")

}

type apiConnFedTest struct{}

func (apiConnFedTest) Run()                          { return }
func (apiConnFedTest) Stop() error                   { return nil }
func (apiConnFedTest) ServiceList() []*api.Service   { return []*api.Service{} }
func (apiConnFedTest) PodIndex(string) []interface{} { return nil }

func (apiConnFedTest) EndpointsList() api.EndpointsList {
	n := "test.node.foo.bar"
	return api.EndpointsList{
		Items: []api.Endpoints{
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

func (apiConnFedTest) GetNodeByName(name string) (api.Node, error) {
	if name != "test.node.foo.bar" {
		return api.Node{}, nil
	}
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

func testFederationCNAMERecord(t *testing.T, k Kubernetes, input recordRequest, expected msg.Service) {
	svc := k.federationCNAMERecord(input)

	if expected.Host != svc.Host {
		t.Errorf("For '%v', expected Host result '%v'. Instead got result '%v'.", input, expected.Host, svc.Host)
	}
	if expected.Key != svc.Key {
		t.Errorf("For '%v', expected Key result '%v'. Instead got result '%v'.", input, expected.Key, svc.Key)
	}
}

func TestFederationCNAMERecord(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs."}}
	k.Federations = []Federation{{name: "fed", zone: "era.tion.com"}}
	k.APIConn = apiConnFedTest{}
	k.interfaceAddrsFunc = func() net.IP { return net.ParseIP("10.9.8.7") }

	m := new(dns.Msg)
	state := request.Request{Zone: "inter.webs.", Req: m}

	m.SetQuestion("s1.ns.fed.svc.inter.webs.", dns.TypeA)
	r, _ := k.parseRequest(state)
	testFederationCNAMERecord(t, k, r, msg.Service{Key: "/coredns/webs/inter/svc/fed/ns/s1", Host: "s1.ns.fed.svc.fd-az.fd-r.era.tion.com"})

	m.SetQuestion("ep1.s1.ns.fed.svc.inter.webs.", dns.TypeA)
	r, _ = k.parseRequest(state)
	testFederationCNAMERecord(t, k, r, msg.Service{Key: "/coredns/webs/inter/svc/fed/ns/s1/ep1", Host: "ep1.s1.ns.fed.svc.fd-az.fd-r.era.tion.com"})

	m.SetQuestion("ep1.s1.ns.foo.svc.inter.webs.", dns.TypeA)
	r, _ = k.parseRequest(state)
	testFederationCNAMERecord(t, k, r, msg.Service{Key: "", Host: ""})
}
