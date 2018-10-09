package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/watch"

	api "k8s.io/api/core/v1"
)

type APIConnTest struct{}

func (APIConnTest) HasSynced() bool                          { return true }
func (APIConnTest) Run()                                     { return }
func (APIConnTest) Stop() error                              { return nil }
func (APIConnTest) PodIndex(string) []*object.Pod            { return nil }
func (APIConnTest) SvcIndex(string) []*object.Service        { return nil }
func (APIConnTest) SvcIndexReverse(string) []*object.Service { return nil }
func (APIConnTest) EpIndex(string) []*object.Endpoints       { return nil }
func (APIConnTest) EndpointsList() []*object.Endpoints       { return nil }
func (APIConnTest) Modified() int64                          { return 0 }
func (APIConnTest) SetWatchChan(watch.Chan)                  {}
func (APIConnTest) Watch(string) error                       { return nil }
func (APIConnTest) StopWatching(string)                      {}

func (APIConnTest) ServiceList() []*object.Service {
	svcs := []*object.Service{
		{
			Name:      "dns-service",
			Namespace: "kube-system",
			ClusterIP: "10.0.0.111",
		},
	}
	return svcs
}

func (APIConnTest) EpIndexReverse(string) []*object.Endpoints {
	eps := []*object.Endpoints{
		{
			Subsets: []object.EndpointSubset{
				{
					Addresses: []object.EndpointAddress{
						{
							IP: "127.0.0.1",
						},
					},
				},
			},
			Name:      "dns-service",
			Namespace: "kube-system",
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
