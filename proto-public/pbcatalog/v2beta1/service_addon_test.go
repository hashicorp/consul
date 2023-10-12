package catalogv2beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsMeshEnabled(t *testing.T) {
	cases := map[string]struct {
		service *Service
		exp     bool
	}{
		"nil": {service: nil, exp: false},
		"no ports": {
			service: &Service{},
			exp:     false,
		},
		"no mesh ports": {
			service: &Service{
				Ports: []*ServicePort{
					{
						TargetPort: "foo",
						Protocol:   Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "bar",
						Protocol:   Protocol_PROTOCOL_TCP,
					},
				},
			},
			exp: false,
		},
		"with mesh ports": {
			service: &Service{
				Ports: []*ServicePort{
					{
						TargetPort: "foo",
						Protocol:   Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "bar",
						Protocol:   Protocol_PROTOCOL_TCP,
					},
					{
						TargetPort: "baz",
						Protocol:   Protocol_PROTOCOL_MESH,
					},
				},
			},
			exp: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.exp, c.service.IsMeshEnabled())
		})
	}
}

func TestFindServicePort(t *testing.T) {
	cases := map[string]struct {
		service *Service
		port    string
		exp     *ServicePort
	}{
		"nil": {service: nil, port: "foo", exp: nil},
		"no ports": {
			service: &Service{},
			port:    "foo",
			exp:     nil,
		},
		"non-existing port": {
			service: &Service{
				Ports: []*ServicePort{
					{
						TargetPort: "foo",
						Protocol:   Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "bar",
						Protocol:   Protocol_PROTOCOL_TCP,
					},
				},
			},
			port: "not-found",
			exp:  nil,
		},
		"existing port": {
			service: &Service{
				Ports: []*ServicePort{
					{
						TargetPort: "foo",
						Protocol:   Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "bar",
						Protocol:   Protocol_PROTOCOL_TCP,
					},
					{
						TargetPort: "baz",
						Protocol:   Protocol_PROTOCOL_MESH,
					},
				},
			},
			port: "bar",
			exp: &ServicePort{
				TargetPort: "bar",
				Protocol:   Protocol_PROTOCOL_TCP,
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.exp, c.service.FindServicePort(c.port))
		})
	}
}
