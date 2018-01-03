package kubernetes

import (
	"testing"

	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type APIConnTest struct{}

func (APIConnTest) HasSynced() bool                       { return true }
func (APIConnTest) Run()                                  { return }
func (APIConnTest) Stop() error                           { return nil }
func (APIConnTest) PodIndex(string) []*api.Pod            { return nil }
func (APIConnTest) SvcIndex(string) []*api.Service        { return nil }
func (APIConnTest) SvcIndexReverse(string) []*api.Service { return nil }
func (APIConnTest) EpIndex(string) []*api.Endpoints       { return nil }
func (APIConnTest) EndpointsList() []*api.Endpoints       { return nil }

func (APIConnTest) ServiceList() []*api.Service {
	svcs := []*api.Service{
		{
			ObjectMeta: meta.ObjectMeta{
				Name:      "dns-service",
				Namespace: "kube-system",
			},
			Spec: api.ServiceSpec{
				ClusterIP: "10.0.0.111",
			},
		},
	}
	return svcs
}

func (APIConnTest) EpIndexReverse(string) []*api.Endpoints {
	eps := []*api.Endpoints{
		{
			Subsets: []api.EndpointSubset{
				{
					Addresses: []api.EndpointAddress{
						{
							IP: "127.0.0.1",
						},
					},
				},
			},
			ObjectMeta: meta.ObjectMeta{
				Name:      "dns-service",
				Namespace: "kube-system",
			},
		},
	}
	return eps
}

func (APIConnTest) GetNodeByName(name string) (*api.Node, error) { return &api.Node{}, nil }
func (APIConnTest) GetNamespaceByName(name string) (*api.Namespace, error) {
	return &api.Namespace{}, nil
}

func TestNsAddr(t *testing.T) {

	k := New([]string{"inter.webs.test."})
	k.APIConn = &APIConnTest{}

	cdr := k.nsAddr()
	expected := "10.0.0.111"

	if cdr.A.String() != expected {
		t.Errorf("Expected A to be %q, got %q", expected, cdr.A.String())
	}
	expected = "dns-service.kube-system.svc."
	if cdr.Hdr.Name != expected {
		t.Errorf("Expected Hdr.Name to be %q, got %q", expected, cdr.Hdr.Name)
	}
}
