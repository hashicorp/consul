package kubernetes

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
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

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func BenchmarkController(b *testing.B) {
	client := fake.NewSimpleClientset()
	dco := dnsControlOpts{
		zones: []string{"cluster.local."},
	}
	controller := newdnsController(client, dco)
	cidr := "10.0.0.0/19"

	// Add resources
	generateEndpoints(cidr, client)
	generateSvcs(cidr, "all", client)
	m := new(dns.Msg)
	m.SetQuestion("svc1.testns.svc.cluster.local.", dns.TypeA)
	k := New([]string{"cluster.local."})
	k.APIConn = controller
	ctx := context.Background()
	rw := &test.ResponseWriter{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k.ServeDNS(ctx, rw, m)
	}
}

func generateEndpoints(cidr string, client kubernetes.Interface) {
	// https://groups.google.com/d/msg/golang-nuts/zlcYA4qk-94/TWRFHeXJCcYJ
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Fatal(err)
	}

	count := 1
	ep := &api.Endpoints{
		Subsets: []api.EndpointSubset{{
			Ports: []api.EndpointPort{
				{
					Port:     80,
					Protocol: "tcp",
					Name:     "http",
				},
			},
		}},
		ObjectMeta: meta.ObjectMeta{
			Namespace: "testns",
		},
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ep.Subsets[0].Addresses = []api.EndpointAddress{
			{
				IP:       ip.String(),
				Hostname: "foo" + strconv.Itoa(count),
			},
		}
		ep.ObjectMeta.Name = "svc" + strconv.Itoa(count)
		_, err = client.Core().Endpoints("testns").Create(ep)
		count += 1
	}
}

func generateSvcs(cidr string, svcType string, client kubernetes.Interface) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Fatal(err)
	}

	count := 1
	switch svcType {
	case "clusterip":
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			createClusterIPSvc(count, client, ip)
			count += 1
		}
	case "headless":
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			createHeadlessSvc(count, client, ip)
			count += 1
		}
	case "external":
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			createExternalSvc(count, client, ip)
			count += 1
		}
	default:
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			if count%3 == 0 {
				createClusterIPSvc(count, client, ip)
			} else if count%3 == 1 {
				createHeadlessSvc(count, client, ip)
			} else if count%3 == 2 {
				createExternalSvc(count, client, ip)
			}
			count += 1
		}
	}
}

func createClusterIPSvc(suffix int, client kubernetes.Interface, ip net.IP) {
	client.Core().Services("testns").Create(&api.Service{
		ObjectMeta: meta.ObjectMeta{
			Name:      "svc" + strconv.Itoa(suffix),
			Namespace: "testns",
		},
		Spec: api.ServiceSpec{
			ClusterIP: ip.String(),
			Ports: []api.ServicePort{{
				Name:     "http",
				Protocol: "tcp",
				Port:     80,
			}},
		},
	})
}

func createHeadlessSvc(suffix int, client kubernetes.Interface, ip net.IP) {
	client.Core().Services("testns").Create(&api.Service{
		ObjectMeta: meta.ObjectMeta{
			Name:      "hdls" + strconv.Itoa(suffix),
			Namespace: "testns",
		},
		Spec: api.ServiceSpec{
			ClusterIP: api.ClusterIPNone,
		},
	})
}

func createExternalSvc(suffix int, client kubernetes.Interface, ip net.IP) {
	client.Core().Services("testns").Create(&api.Service{
		ObjectMeta: meta.ObjectMeta{
			Name:      "external" + strconv.Itoa(suffix),
			Namespace: "testns",
		},
		Spec: api.ServiceSpec{
			ExternalName: "coredns" + strconv.Itoa(suffix) + ".io",
			Ports: []api.ServicePort{{
				Name:     "http",
				Protocol: "tcp",
				Port:     80,
			}},
			Type: api.ServiceTypeExternalName,
		},
	})
}
