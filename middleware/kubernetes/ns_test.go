package kubernetes

import (
	"testing"

	"k8s.io/client-go/1.5/pkg/api"
)

type APIConnTest struct{}

func (APIConnTest) Run()                          { return }
func (APIConnTest) Stop() error                   { return nil }
func (APIConnTest) PodIndex(string) []interface{} { return nil }

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

func (APIConnTest) EndpointsList() api.EndpointsList {
	return api.EndpointsList{
		Items: []api.Endpoints{
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
				ObjectMeta: api.ObjectMeta{
					Name:      "dns-service",
					Namespace: "kube-system",
				},
			},
		},
	}
}

func (APIConnTest) GetNodeByName(name string) (api.Node, error) { return api.Node{}, nil }

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
