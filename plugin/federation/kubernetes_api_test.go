package federation

import (
	"github.com/coredns/coredns/plugin/kubernetes"

	"k8s.io/client-go/1.5/pkg/api"
)

type APIConnFederationTest struct{}

func (APIConnFederationTest) Run()        { return }
func (APIConnFederationTest) Stop() error { return nil }

func (APIConnFederationTest) PodIndex(string) []interface{} {
	a := make([]interface{}, 1)
	a[0] = &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Namespace: "podns",
		},
		Status: api.PodStatus{
			PodIP: "10.240.0.1", // Remote IP set in test.ResponseWriter
		},
	}
	return a
}

func (APIConnFederationTest) ServiceList() []*api.Service {
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
				ExternalName: "ext.interwebs.test",
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

func (APIConnFederationTest) EndpointsList() api.EndpointsList {
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
		},
	}
}

func (APIConnFederationTest) GetNodeByName(name string) (api.Node, error) {
	return api.Node{
		ObjectMeta: api.ObjectMeta{
			Name: "test.node.foo.bar",
			Labels: map[string]string{
				kubernetes.LabelRegion: "fd-r",
				kubernetes.LabelZone:   "fd-az",
			},
		},
	}, nil
}
