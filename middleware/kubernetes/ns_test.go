package kubernetes

import "testing"
import "net"

import "github.com/coredns/coredns/middleware/etcd/msg"
import "k8s.io/client-go/1.5/pkg/api"
import "github.com/miekg/dns"

func TestRecordForNS(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs.test"}}
	corednsRecord.Hdr.Name = "coredns.kube-system."
	corednsRecord.A = net.IP("1.2.3.4")
	r, _ := k.parseRequest("inter.webs.test", dns.TypeNS)
	expected := "/coredns/test/webs/inter/kube-system/coredns"

	var svcs []msg.Service
	k.recordsForNS(r, &svcs)
	if svcs[0].Key != expected {
		t.Errorf("Expected  result '%v'. Instead got result '%v'.", expected, svcs[0].Key)
	}
}

func TestDefaultNSMsg(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs.test"}}
	corednsRecord.Hdr.Name = "coredns.kube-system."
	corednsRecord.A = net.IP("1.2.3.4")
	r, _ := k.parseRequest("ns.dns.inter.webs.test", dns.TypeA)
	expected := "/coredns/test/webs/inter/dns/ns"

	svc := k.defaultNSMsg(r)
	if svc.Key != expected {
		t.Errorf("Expected  result '%v'. Instead got result '%v'.", expected, svc.Key)
	}
}

func TestIsDefaultNS(t *testing.T) {
	k := Kubernetes{Zones: []string{"inter.webs.test"}}
	r, _ := k.parseRequest("ns.dns.inter.webs.test", dns.TypeA)

	var name string
	var expected bool

	name = "ns.dns.inter.webs.test"
	expected = true
	if isDefaultNS(name, r) != expected {
		t.Errorf("Expected IsDefaultNS('%v') to be '%v'.", name, expected)
	}
	name = "ns.dns.blah.inter.webs.test"
	expected = false
	if isDefaultNS(name, r) != expected {
		t.Errorf("Expected IsDefaultNS('%v') to be '%v'.", name, expected)
	}
}

type APIConnTest struct{}

func (APIConnTest) Run() {
	return
}

func (APIConnTest) Stop() error {
	return nil
}

func (APIConnTest) ServiceList() []*api.Service {
	svc := api.Service{
		ObjectMeta: api.ObjectMeta{
			Name:      "dns-service",
			Namespace: "kube-system",
		},
		Spec: api.ServiceSpec{
			ClusterIP: "10.0.0.111",
		},
	}

	return []*api.Service{&svc}

}

func (APIConnTest) PodIndex(string) []interface{} {
	return nil
}

func (APIConnTest) EndpointsList() api.EndpointsList {
	return api.EndpointsList{
		Items: []api.Endpoints{
			{
				Subsets: []api.EndpointSubset{
					{
						Addresses: []api.EndpointAddress{
							{
								IP: "172.0.40.10",
							},
						},
					},
				},
				ObjectMeta: api.ObjectMeta{
					Name:      "dns-service",
					Namespace: "kube-system",
				},
			},
		},
	}
}

func (APIConnTest) GetNodeByName(name string) (api.Node, error) { return api.Node{}, nil }

type interfaceAddrsTest struct{}

func (i interfaceAddrsTest) interfaceAddrs() ([]net.Addr, error) {
	_, ipnet, _ := net.ParseCIDR("172.0.40.10/32")
	return []net.Addr{ipnet}, nil
}

func TestDoCoreDNSRecord(t *testing.T) {

	corednsRecord = dns.A{}
	k := Kubernetes{Zones: []string{"inter.webs.test"}}

	k.interfaceAddrs = &interfaceAddrsTest{}
	k.APIConn = &APIConnTest{}

	cdr := k.coreDNSRecord()

	expected := "10.0.0.111"

	if cdr.A.String() != expected {
		t.Errorf("Expected A to be '%v', got '%v'", expected, cdr.A.String())
	}
	expected = "dns-service.kube-system.svc."
	if cdr.Hdr.Name != expected {
		t.Errorf("Expected Hdr.Name to be '%v', got '%v'", expected, cdr.Hdr.Name)
	}
}
