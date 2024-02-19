// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceIsMeshEnabled(t *testing.T) {
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

func TestFindPort(t *testing.T) {
	cases := map[string]struct {
		service         *Service
		port            string
		expById         *ServicePort
		expByTargetPort *ServicePort
	}{
		"nil": {service: nil, port: "foo", expById: nil, expByTargetPort: nil},
		"no ports": {
			service:         &Service{},
			port:            "foo",
			expById:         nil,
			expByTargetPort: nil,
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
			port:            "not-found",
			expById:         nil,
			expByTargetPort: nil,
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
			expById: &ServicePort{
				TargetPort: "bar",
				Protocol:   Protocol_PROTOCOL_TCP,
			},
			expByTargetPort: &ServicePort{
				TargetPort: "bar",
				Protocol:   Protocol_PROTOCOL_TCP,
			},
		},
		"existing port by virtual port": {
			service: &Service{
				Ports: []*ServicePort{
					{
						TargetPort:  "foo",
						VirtualPort: 8080,
						Protocol:    Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort:  "bar",
						VirtualPort: 8081,
						Protocol:    Protocol_PROTOCOL_TCP,
					},
					{
						TargetPort:  "baz",
						VirtualPort: 8081,
						Protocol:    Protocol_PROTOCOL_MESH,
					},
				},
			},
			port: "8081",
			expById: &ServicePort{
				TargetPort:  "bar",
				VirtualPort: 8081,
				Protocol:    Protocol_PROTOCOL_TCP,
			},
			expByTargetPort: nil,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.expById, c.service.FindPortByID(c.port))
			require.Equal(t, c.expByTargetPort, c.service.FindTargetPort(c.port))
		})
	}
}

func TestMatchesPortId(t *testing.T) {
	testPort := &ServicePort{VirtualPort: 8080, TargetPort: "http"}

	cases := map[string]struct {
		port     *ServicePort
		id       string
		expected bool
	}{
		"nil":   {port: nil, id: "foo", expected: false},
		"empty": {port: testPort, id: "", expected: false},
		"non-existing virtual port": {
			port:     testPort,
			id:       "9090",
			expected: false,
		},
		"non-existing target port": {
			port:     testPort,
			id:       "other-port",
			expected: false,
		},
		"existing virtual port": {
			port:     testPort,
			id:       "8080",
			expected: true,
		},
		"existing target port": {
			port:     testPort,
			id:       "http",
			expected: true,
		},
		"virtual and target mismatch": {
			port:     &ServicePort{VirtualPort: 8080, TargetPort: "9090"},
			id:       "9090",
			expected: false,
		},
		"virtual and target match": {
			port:     &ServicePort{VirtualPort: 9090, TargetPort: "9090"},
			id:       "9090",
			expected: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.expected, c.port.MatchesPortId(c.id))
		})
	}
}
