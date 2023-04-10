// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

func TestConfigEntries_ListRelatedServices_AndACLs(t *testing.T) {
	// This test tests both of these because they are related functions.

	newAuthz := func(t *testing.T, src string) acl.Authorizer {
		policy, err := acl.NewPolicyFromSource(src, nil, nil)
		require.NoError(t, err)

		authorizer, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)
		return authorizer
	}

	newServiceACL := func(t *testing.T, canRead, canWrite []string) acl.Authorizer {
		var buf bytes.Buffer
		for _, s := range canRead {
			buf.WriteString(fmt.Sprintf("service %q { policy = %q }\n", s, "read"))
		}
		for _, s := range canWrite {
			buf.WriteString(fmt.Sprintf("service %q { policy = %q }\n", s, "write"))
		}

		policy, err := acl.NewPolicyFromSource(buf.String(), nil, nil)
		require.NoError(t, err)

		authorizer, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)
		return authorizer
	}

	newServiceAndOperatorACL := func(t *testing.T, service, operator string) acl.Authorizer {
		switch {
		case service != "" && operator != "":
			return newAuthz(t, fmt.Sprintf(`service "test" { policy = %q } operator = %q`, service, operator))
		case service == "" && operator != "":
			return newAuthz(t, fmt.Sprintf(`operator = %q`, operator))
		case service != "" && operator == "":
			return newAuthz(t, fmt.Sprintf(`service "test" { policy = %q }`, service))
		default:
			t.Fatalf("one of these should be set")
			return nil
		}
	}

	newServiceAndMeshACL := func(t *testing.T, service, mesh string) acl.Authorizer {
		switch {
		case service != "" && mesh != "":
			return newAuthz(t, fmt.Sprintf(`service "test" { policy = %q } mesh = %q`, service, mesh))
		case service == "" && mesh != "":
			return newAuthz(t, fmt.Sprintf(`mesh = %q`, mesh))
		case service != "" && mesh == "":
			return newAuthz(t, fmt.Sprintf(`service "test" { policy = %q }`, service))
		default:
			t.Fatalf("one of these should be set")
			return nil
		}
	}

	type testACL = configEntryTestACL
	type testcase = configEntryACLTestCase

	defaultDenyCase := testACL{
		name:       "deny",
		authorizer: newServiceACL(t, nil, nil),
		canRead:    false,
		canWrite:   false,
	}
	readTestCase := testACL{
		name:       "can read test",
		authorizer: newServiceACL(t, []string{"test"}, nil),
		canRead:    true,
		canWrite:   false,
	}
	writeTestCase := testACL{
		name:       "can write test",
		authorizer: newServiceACL(t, nil, []string{"test"}),
		canRead:    true,
		canWrite:   true,
	}
	writeTestCaseDenied := testACL{
		name:       "cannot write test",
		authorizer: newServiceACL(t, nil, []string{"test"}),
		canRead:    true,
		canWrite:   false,
	}

	cases := []testcase{
		{
			name: "resolver: self",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
			},
			expectServices: nil,
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCase,
			},
		},
		{
			name: "resolver: redirect",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service: "other",
				},
			},
			expectServices: []ServiceID{NewServiceID("other", nil)},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with other:read)",
					authorizer: newServiceACL(t, []string{"other"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "resolver: failover",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"foo": {OnlyPassing: true},
					"bar": {OnlyPassing: true},
				},
				Failover: map[string]ServiceResolverFailover{
					"foo": {
						Service: "other1",
					},
					"bar": {
						Service: "other2",
					},
				},
			},
			expectServices: []ServiceID{NewServiceID("other1", nil), NewServiceID("other2", nil)},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with other1:read and other2:read)",
					authorizer: newServiceACL(t, []string{"other1", "other2"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "resolver: failover with targets",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Targets: []ServiceResolverFailoverTarget{
							{Service: "other1"},
							{Datacenter: "dc2"},
							{Peer: "cluster-01"},
						},
					},
				},
			},
			expectServices: []ServiceID{NewServiceID("other1", nil)},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with other1:read)",
					authorizer: newServiceACL(t, []string{"other1"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "splitter: self",
			entry: &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "test",
				Splits: []ServiceSplit{
					{Weight: 100},
				},
			},
			expectServices: nil,
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCase,
			},
		},
		{
			name: "splitter: some",
			entry: &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "test",
				Splits: []ServiceSplit{
					{Weight: 25, Service: "b"},
					{Weight: 25, Service: "a"},
					{Weight: 50, Service: "c"},
				},
			},
			expectServices: []ServiceID{NewServiceID("a", nil), NewServiceID("b", nil), NewServiceID("c", nil)},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with a:read, b:read, and c:read)",
					authorizer: newServiceACL(t, []string{"a", "b", "c"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "router: self",
			entry: &ServiceRouterConfigEntry{
				Kind: ServiceRouter,
				Name: "test",
			},
			expectServices: []ServiceID{NewServiceID("test", nil)},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCase,
			},
		},
		{
			name: "router: some",
			entry: &ServiceRouterConfigEntry{
				Kind: ServiceRouter,
				Name: "test",
				Routes: []ServiceRoute{
					{
						Match: &ServiceRouteMatch{HTTP: &ServiceRouteHTTPMatch{
							PathPrefix: "/foo",
						}},
						Destination: &ServiceRouteDestination{
							Service: "foo",
						},
					},
					{
						Match: &ServiceRouteMatch{HTTP: &ServiceRouteHTTPMatch{
							PathPrefix: "/bar",
						}},
						Destination: &ServiceRouteDestination{
							Service: "bar",
						},
					},
				},
			},
			expectServices: []ServiceID{NewServiceID("bar", nil), NewServiceID("foo", nil), NewServiceID("test", nil)},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with foo:read and bar:read)",
					authorizer: newServiceACL(t, []string{"foo", "bar"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name:  "ingress-gateway",
			entry: &IngressGatewayConfigEntry{Name: "test"},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator deny",
					authorizer: newServiceAndOperatorACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator deny",
					authorizer: newServiceAndOperatorACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator deny",
					authorizer: newServiceAndOperatorACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh deny",
					authorizer: newServiceAndMeshACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh deny",
					authorizer: newServiceAndMeshACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh deny",
					authorizer: newServiceAndMeshACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator read",
					authorizer: newServiceAndOperatorACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator read",
					authorizer: newServiceAndOperatorACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator read",
					authorizer: newServiceAndOperatorACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator write",
					authorizer: newServiceAndOperatorACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   true,
				},
				{
					name:       "service read and operator write",
					authorizer: newServiceAndOperatorACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and operator write",
					authorizer: newServiceAndOperatorACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},

				{
					name:       "service deny and mesh read",
					authorizer: newServiceAndMeshACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh read",
					authorizer: newServiceAndMeshACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh read",
					authorizer: newServiceAndMeshACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh write",
					authorizer: newServiceAndMeshACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   true,
				},
				{
					name:       "service read and mesh write",
					authorizer: newServiceAndMeshACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and mesh write",
					authorizer: newServiceAndMeshACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "api-gateway",
			entry: &APIGatewayConfigEntry{
				Name: "test",
				Listeners: []APIGatewayListener{
					{
						Name:     "test",
						Port:     100,
						Protocol: "http",
					},
				},
			},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator deny",
					authorizer: newServiceAndOperatorACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator deny",
					authorizer: newServiceAndOperatorACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator deny",
					authorizer: newServiceAndOperatorACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh deny",
					authorizer: newServiceAndMeshACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh deny",
					authorizer: newServiceAndMeshACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh deny",
					authorizer: newServiceAndMeshACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator read",
					authorizer: newServiceAndOperatorACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator read",
					authorizer: newServiceAndOperatorACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator read",
					authorizer: newServiceAndOperatorACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator write",
					authorizer: newServiceAndOperatorACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   true,
				},
				{
					name:       "service read and operator write",
					authorizer: newServiceAndOperatorACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and operator write",
					authorizer: newServiceAndOperatorACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},

				{
					name:       "service deny and mesh read",
					authorizer: newServiceAndMeshACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh read",
					authorizer: newServiceAndMeshACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh read",
					authorizer: newServiceAndMeshACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh write",
					authorizer: newServiceAndMeshACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   true,
				},
				{
					name:       "service read and mesh write",
					authorizer: newServiceAndMeshACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and mesh write",
					authorizer: newServiceAndMeshACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name:  "inline-certificate",
			entry: &InlineCertificateConfigEntry{Name: "test", Certificate: validCertificate, PrivateKey: validPrivateKey},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator deny",
					authorizer: newServiceAndOperatorACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator deny",
					authorizer: newServiceAndOperatorACL(t, "read", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service write and operator deny",
					authorizer: newServiceAndOperatorACL(t, "write", "deny"),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh deny",
					authorizer: newServiceAndMeshACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh deny",
					authorizer: newServiceAndMeshACL(t, "read", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service write and mesh deny",
					authorizer: newServiceAndMeshACL(t, "write", "deny"),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator read",
					authorizer: newServiceAndOperatorACL(t, "deny", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service read and operator read",
					authorizer: newServiceAndOperatorACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator read",
					authorizer: newServiceAndOperatorACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator write",
					authorizer: newServiceAndOperatorACL(t, "deny", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service read and operator write",
					authorizer: newServiceAndOperatorACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and operator write",
					authorizer: newServiceAndOperatorACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},

				{
					name:       "service deny and mesh read",
					authorizer: newServiceAndMeshACL(t, "deny", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service read and mesh read",
					authorizer: newServiceAndMeshACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh read",
					authorizer: newServiceAndMeshACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh write",
					authorizer: newServiceAndMeshACL(t, "deny", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service read and mesh write",
					authorizer: newServiceAndMeshACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and mesh write",
					authorizer: newServiceAndMeshACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name:  "http-route",
			entry: &HTTPRouteConfigEntry{Name: "test"},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator deny",
					authorizer: newServiceAndOperatorACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator deny",
					authorizer: newServiceAndOperatorACL(t, "read", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service write and operator deny",
					authorizer: newServiceAndOperatorACL(t, "write", "deny"),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh deny",
					authorizer: newServiceAndMeshACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh deny",
					authorizer: newServiceAndMeshACL(t, "read", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service write and mesh deny",
					authorizer: newServiceAndMeshACL(t, "write", "deny"),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator read",
					authorizer: newServiceAndOperatorACL(t, "deny", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service read and operator read",
					authorizer: newServiceAndOperatorACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator read",
					authorizer: newServiceAndOperatorACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator write",
					authorizer: newServiceAndOperatorACL(t, "deny", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service read and operator write",
					authorizer: newServiceAndOperatorACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and operator write",
					authorizer: newServiceAndOperatorACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},

				{
					name:       "service deny and mesh read",
					authorizer: newServiceAndMeshACL(t, "deny", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service read and mesh read",
					authorizer: newServiceAndMeshACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh read",
					authorizer: newServiceAndMeshACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh write",
					authorizer: newServiceAndMeshACL(t, "deny", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service read and mesh write",
					authorizer: newServiceAndMeshACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and mesh write",
					authorizer: newServiceAndMeshACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name:  "tcp-route",
			entry: &TCPRouteConfigEntry{Name: "test"},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator deny",
					authorizer: newServiceAndOperatorACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator deny",
					authorizer: newServiceAndOperatorACL(t, "read", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service write and operator deny",
					authorizer: newServiceAndOperatorACL(t, "write", "deny"),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh deny",
					authorizer: newServiceAndMeshACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh deny",
					authorizer: newServiceAndMeshACL(t, "read", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service write and mesh deny",
					authorizer: newServiceAndMeshACL(t, "write", "deny"),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator read",
					authorizer: newServiceAndOperatorACL(t, "deny", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service read and operator read",
					authorizer: newServiceAndOperatorACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator read",
					authorizer: newServiceAndOperatorACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator write",
					authorizer: newServiceAndOperatorACL(t, "deny", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service read and operator write",
					authorizer: newServiceAndOperatorACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and operator write",
					authorizer: newServiceAndOperatorACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},

				{
					name:       "service deny and mesh read",
					authorizer: newServiceAndMeshACL(t, "deny", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service read and mesh read",
					authorizer: newServiceAndMeshACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh read",
					authorizer: newServiceAndMeshACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh write",
					authorizer: newServiceAndMeshACL(t, "deny", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service read and mesh write",
					authorizer: newServiceAndMeshACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and mesh write",
					authorizer: newServiceAndMeshACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name:  "bound-api-gateway",
			entry: &BoundAPIGatewayConfigEntry{Name: "test"},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator deny",
					authorizer: newServiceAndOperatorACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator deny",
					authorizer: newServiceAndOperatorACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator deny",
					authorizer: newServiceAndOperatorACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh deny",
					authorizer: newServiceAndMeshACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh deny",
					authorizer: newServiceAndMeshACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh deny",
					authorizer: newServiceAndMeshACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator read",
					authorizer: newServiceAndOperatorACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator read",
					authorizer: newServiceAndOperatorACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator read",
					authorizer: newServiceAndOperatorACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator write",
					authorizer: newServiceAndOperatorACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator write",
					authorizer: newServiceAndOperatorACL(t, "read", "write"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator write",
					authorizer: newServiceAndOperatorACL(t, "write", "write"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh read",
					authorizer: newServiceAndMeshACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh read",
					authorizer: newServiceAndMeshACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh read",
					authorizer: newServiceAndMeshACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh write",
					authorizer: newServiceAndMeshACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh write",
					authorizer: newServiceAndMeshACL(t, "read", "write"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh write",
					authorizer: newServiceAndMeshACL(t, "write", "write"),
					canRead:    true,
					canWrite:   false,
				},
			},
		},
		{
			name:  "terminating-gateway",
			entry: &TerminatingGatewayConfigEntry{Name: "test"},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    false,
					canWrite:   false,
				},

				{
					name:       "service deny and operator deny",
					authorizer: newServiceAndOperatorACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator deny",
					authorizer: newServiceAndOperatorACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator deny",
					authorizer: newServiceAndOperatorACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh deny",
					authorizer: newServiceAndMeshACL(t, "deny", "deny"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh deny",
					authorizer: newServiceAndMeshACL(t, "read", "deny"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh deny",
					authorizer: newServiceAndMeshACL(t, "write", "deny"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator read",
					authorizer: newServiceAndOperatorACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and operator read",
					authorizer: newServiceAndOperatorACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and operator read",
					authorizer: newServiceAndOperatorACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and operator write",
					authorizer: newServiceAndOperatorACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   true,
				},
				{
					name:       "service read and operator write",
					authorizer: newServiceAndOperatorACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and operator write",
					authorizer: newServiceAndOperatorACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},

				{
					name:       "service deny and mesh read",
					authorizer: newServiceAndMeshACL(t, "deny", "read"),
					canRead:    false,
					canWrite:   false,
				},
				{
					name:       "service read and mesh read",
					authorizer: newServiceAndMeshACL(t, "read", "read"),
					canRead:    true,
					canWrite:   false,
				},
				{
					name:       "service write and mesh read",
					authorizer: newServiceAndMeshACL(t, "write", "read"),
					canRead:    true,
					canWrite:   false,
				},

				{
					name:       "service deny and mesh write",
					authorizer: newServiceAndMeshACL(t, "deny", "write"),
					canRead:    false,
					canWrite:   true,
				},
				{
					name:       "service read and mesh write",
					authorizer: newServiceAndMeshACL(t, "read", "write"),
					canRead:    true,
					canWrite:   true,
				},
				{
					name:       "service write and mesh write",
					authorizer: newServiceAndMeshACL(t, "write", "write"),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
	}

	testConfigEntries_ListRelatedServices_AndACLs(t, cases)
}

func TestServiceResolverConfigEntry(t *testing.T) {

	type testcase struct {
		name         string
		entry        *ServiceResolverConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceResolverConfigEntry)
	}

	cases := []testcase{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceResolverConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name: "empty",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
			},
		},
		{
			name: "empty subset name",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"": {OnlyPassing: true},
				},
			},
			validateErr: "Subset defined with empty name",
		},
		{
			name: "invalid boolean expression subset filter",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "random string"},
				},
			},
			validateErr: `Filter for subset "v1" is not a valid expression`,
		},
		{
			name: "default subset does not exist",
			entry: &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test",
				DefaultSubset: "gone",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
			},
			validateErr: `DefaultSubset "gone" is not a valid subset`,
		},
		{
			name: "default subset does exist",
			entry: &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test",
				DefaultSubset: "v1",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
			},
		},
		{
			name: "empty redirect",
			entry: &ServiceResolverConfigEntry{
				Kind:     ServiceResolver,
				Name:     "test",
				Redirect: &ServiceResolverRedirect{},
			},
			validateErr: "Redirect is empty",
		},
		{
			name: "empty redirect",
			entry: &ServiceResolverConfigEntry{
				Kind:     ServiceResolver,
				Name:     "test",
				Redirect: &ServiceResolverRedirect{},
			},
			validateErr: "Redirect is empty",
		},
		{
			name: "redirect subset with no service",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					ServiceSubset: "next",
				},
			},
			validateErr: "Redirect.ServiceSubset defined without Redirect.Service",
		},
		{
			name: "self redirect with invalid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service:       "test",
					ServiceSubset: "gone",
				},
			},
			validateErr: `Redirect.ServiceSubset "gone" is not a valid subset of "test"`,
		},
		{
			name: "redirect with peer and subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Peer:          "cluster-01",
					ServiceSubset: "gone",
				},
			},
			validateErr: `Redirect.Peer cannot be set with Redirect.ServiceSubset`,
		},
		{
			name: "redirect with peer and datacenter",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Peer:       "cluster-01",
					Datacenter: "dc2",
				},
			},
			validateErr: `Redirect.Peer cannot be set with Redirect.Datacenter`,
		},
		{
			name: "redirect with peer and datacenter",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Peer: "cluster-01",
				},
			},
			validateErr: `Redirect.Peer defined without Redirect.Service`,
		},
		{
			name: "self redirect with valid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service:       "test",
					ServiceSubset: "v1",
				},
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
			},
		},
		{
			name: "redirect to peer",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service: "other",
					Peer:    "cluster-01",
				},
			},
		},
		{
			name: "simple wildcard failover",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Datacenters: []string{"dc2"},
					},
				},
			},
		},
		{
			name: "failover for missing subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"gone": {
						Datacenters: []string{"dc2"},
					},
				},
			},
			validateErr: `Bad Failover["gone"]: not a valid subset`,
		},
		{
			name: "failover for present subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": {
						Datacenters: []string{"dc2"},
					},
				},
			},
		},
		{
			name: "failover empty",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": {},
				},
			},
			validateErr: `Bad Failover["v1"]: one of Service, ServiceSubset, Namespace, Targets, SamenessGroup, or Datacenters is required`,
		},
		{
			name: "failover to self using invalid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": {
						Service:       "test",
						ServiceSubset: "gone",
					},
				},
			},
			validateErr: `Bad Failover["v1"]: ServiceSubset "gone" is not a valid subset of "test"`,
		},
		{
			name: "failover to self using valid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
					"v2": {Filter: "Service.Meta.version == v2"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": {
						Service:       "test",
						ServiceSubset: "v2",
					},
				},
			},
		},
		{
			name: "failover with empty datacenters in list",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Service:     "backup",
						Datacenters: []string{"", "dc2", "dc3"},
					},
				},
			},
			validateErr: `Bad Failover["*"].Datacenters: found empty datacenter`,
		},
		{
			name: "failover target with an invalid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Targets: []ServiceResolverFailoverTarget{{ServiceSubset: "subset"}},
					},
				},
			},
			validateErr: `Bad Failover["*"].Targets[0]: ServiceSubset "subset" is not a valid subset of "test"`,
		},
		{
			name: "failover targets can't have Peer and ServiceSubset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Targets: []ServiceResolverFailoverTarget{{Peer: "cluster-01", ServiceSubset: "subset"}},
					},
				},
			},
			validateErr: `Bad Failover["*"].Targets[0]: Peer cannot be set with ServiceSubset`,
		},
		{
			name: "failover targets can't have Peer and Datacenter",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Targets: []ServiceResolverFailoverTarget{{Peer: "cluster-01", Datacenter: "dc1"}},
					},
				},
			},
			validateErr: `Bad Failover["*"].Targets[0]: Peer cannot be set with Datacenter`,
		},
		{
			name: "failover Targets cannot be set with Datacenters",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Datacenters: []string{"a"},
						Targets:     []ServiceResolverFailoverTarget{{Peer: "cluster-01"}},
					},
				},
			},
			validateErr: `Bad Failover["*"]: Targets cannot be set with Datacenters`,
		},
		{
			name: "failover Targets cannot be set with ServiceSubset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						ServiceSubset: "v2",
						Targets:       []ServiceResolverFailoverTarget{{Peer: "cluster-01"}},
					},
				},
				Subsets: map[string]ServiceResolverSubset{
					"v2": {Filter: "Service.Meta.version == v2"},
				},
			},
			validateErr: `Bad Failover["*"]: Targets cannot be set with ServiceSubset`,
		},
		{
			name: "failover Targets cannot be set with Service",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Service: "another-service",
						Targets: []ServiceResolverFailoverTarget{{Peer: "cluster-01"}},
					},
				},
				Subsets: map[string]ServiceResolverSubset{
					"v2": {Filter: "Service.Meta.version == v2"},
				},
			},
			validateErr: `Bad Failover["*"]: Targets cannot be set with Service`,
		},
		{
			name: "complicated failover targets",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": {
						Targets: []ServiceResolverFailoverTarget{
							{Peer: "cluster-01", Service: "test-v2"},
							{Service: "test-v2", ServiceSubset: "test"},
							{Datacenter: "dc2"},
						},
					},
				},
			},
		},
		{
			name: "bad connect timeout",
			entry: &ServiceResolverConfigEntry{
				Kind:           ServiceResolver,
				Name:           "test",
				ConnectTimeout: -1 * time.Second,
			},
			validateErr: "Bad ConnectTimeout",
		},
	}

	// Bulk add a bunch of similar validation cases.
	for _, invalidSubset := range invalidSubsetNames {
		tc := testcase{
			name: "invalid subset name: " + invalidSubset,
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					invalidSubset: {OnlyPassing: true},
				},
			},
			validateErr: fmt.Sprintf("Subset %q is invalid", invalidSubset),
		}
		cases = append(cases, tc)
	}

	for _, goodSubset := range validSubsetNames {
		tc := testcase{
			name: "valid subset name: " + goodSubset,
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					goodSubset: {OnlyPassing: true},
				},
			},
		}
		cases = append(cases, tc)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceResolverConfigEntry_LoadBalancer(t *testing.T) {

	type testcase struct {
		name         string
		entry        *ServiceResolverConfigEntry
		normalizeErr string
		validateErr  string

		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceResolverConfigEntry)
	}

	cases := []testcase{
		{
			name: "empty policy is valid",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: "",
				},
			},
		},
		{
			name: "supported policy",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyRandom,
				},
			},
		},
		{
			name: "unsupported policy",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: "fake-policy",
				},
			},
			validateErr: `"fake-policy" is not supported`,
		},
		{
			name: "bad policy for least request config",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy:             LBPolicyRingHash,
					LeastRequestConfig: &LeastRequestConfig{ChoiceCount: 10},
				},
			},
			validateErr: `LeastRequestConfig specified for incompatible load balancing policy`,
		},
		{
			name: "bad policy for ring hash config",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy:         LBPolicyLeastRequest,
					RingHashConfig: &RingHashConfig{MinimumRingSize: 1024},
				},
			},
			validateErr: `RingHashConfig specified for incompatible load balancing policy`,
		},
		{
			name: "good policy for ring hash config",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy:         LBPolicyRingHash,
					RingHashConfig: &RingHashConfig{MinimumRingSize: 1024},
				},
			},
		},
		{
			name: "good policy for least request config",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy:             LBPolicyLeastRequest,
					LeastRequestConfig: &LeastRequestConfig{ChoiceCount: 2},
				},
			},
		},
		{
			name: "empty policy is not defaulted",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: "",
				},
			},
			check: func(t *testing.T, entry *ServiceResolverConfigEntry) {
				require.Equal(t, "", entry.LoadBalancer.Policy)
			},
		},
		{
			name: "empty policy with hash policy",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: "",
					HashPolicies: []HashPolicy{
						{
							SourceIP: true,
						},
					},
				},
			},
			validateErr: `HashPolicies specified for non-hash-based Policy`,
		},
		{
			name: "cookie config with header policy",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							Field:      HashPolicyHeader,
							FieldValue: "x-user-id",
							CookieConfig: &CookieConfig{
								TTL:  10 * time.Second,
								Path: "/root",
							},
						},
					},
				},
			},
			validateErr: `cookie_config provided for "header"`,
		},
		{
			name: "cannot generate session cookie with ttl",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							Field:      HashPolicyCookie,
							FieldValue: "good-cookie",
							CookieConfig: &CookieConfig{
								Session: true,
								TTL:     10 * time.Second,
							},
						},
					},
				},
			},
			validateErr: `a session cookie cannot have an associated TTL`,
		},
		{
			name: "valid cookie policy",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							Field:      HashPolicyCookie,
							FieldValue: "good-cookie",
							CookieConfig: &CookieConfig{
								TTL:  10 * time.Second,
								Path: "/oven",
							},
						},
					},
				},
			},
		},
		{
			name: "supported match field",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							Field:      "header",
							FieldValue: "X-Consul-Token",
						},
					},
				},
			},
		},
		{
			name: "unsupported match field",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							Field: "fake-field",
						},
					},
				},
			},
			validateErr: `"fake-field" is not a supported field`,
		},
		{
			name: "cannot match on source address and custom field",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							Field:    "header",
							SourceIP: true,
						},
					},
				},
			},
			validateErr: `A single hash policy cannot hash both a source address and a "header"`,
		},
		{
			name: "matchvalue not compatible with source address",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							FieldValue: "X-Consul-Token",
							SourceIP:   true,
						},
					},
				},
			},
			validateErr: `A FieldValue cannot be specified when hashing SourceIP`,
		},
		{
			name: "field without match value",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							Field: "header",
						},
					},
				},
			},
			validateErr: `Field "header" was specified without a FieldValue`,
		},
		{
			name: "field without match value",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyMaglev,
					HashPolicies: []HashPolicy{
						{
							FieldValue: "my-cookie",
						},
					},
				},
			},
			validateErr: `FieldValue requires a Field to apply to`,
		},
		{
			name: "ring hash kitchen sink",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy:         LBPolicyRingHash,
					RingHashConfig: &RingHashConfig{MaximumRingSize: 10, MinimumRingSize: 2},
					HashPolicies: []HashPolicy{
						{
							Field:      "cookie",
							FieldValue: "my-cookie",
						},
						{
							Field:      "header",
							FieldValue: "alt-header",
							Terminal:   true,
						},
					},
				},
			},
		},
		{
			name: "least request kitchen sink",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				LoadBalancer: &LoadBalancer{
					Policy:             LBPolicyLeastRequest,
					LeastRequestConfig: &LeastRequestConfig{ChoiceCount: 20},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceSplitterConfigEntry(t *testing.T) {

	makesplitter := func(splits ...ServiceSplit) *ServiceSplitterConfigEntry {
		return &ServiceSplitterConfigEntry{
			Kind:   ServiceSplitter,
			Name:   "test",
			Splits: splits,
		}
	}

	makesplit := func(weight float32, service, serviceSubset, namespace string) ServiceSplit {
		return ServiceSplit{
			Weight:        weight,
			Service:       service,
			ServiceSubset: serviceSubset,
			Namespace:     namespace,
		}
	}

	for _, tc := range []struct {
		name         string
		entry        *ServiceSplitterConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceSplitterConfigEntry)
	}{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceSplitterConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name:        "empty",
			entry:       makesplitter(),
			validateErr: "no splits configured",
		},
		{
			name: "1 split",
			entry: makesplitter(
				makesplit(100, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
			},
		},
		{
			name: "1 split not enough weight",
			entry: makesplitter(
				makesplit(99.99, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99.99), entry.Splits[0].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "1 split too much weight",
			entry: makesplitter(
				makesplit(100.01, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100.01), entry.Splits[0].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "2 splits",
			entry: makesplitter(
				makesplit(99, "test", "v1", ""),
				makesplit(1, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99), entry.Splits[0].Weight)
				require.Equal(t, float32(1), entry.Splits[1].Weight)
			},
		},
		{
			name: "2 splits - rounded up to smallest units",
			entry: makesplitter(
				makesplit(99.999, "test", "v1", ""),
				makesplit(0.001, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
				require.Equal(t, float32(0), entry.Splits[1].Weight)
			},
		},
		{
			name: "2 splits not enough weight",
			entry: makesplitter(
				makesplit(99.98, "test", "v1", ""),
				makesplit(0.01, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99.98), entry.Splits[0].Weight)
				require.Equal(t, float32(0.01), entry.Splits[1].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "2 splits too much weight",
			entry: makesplitter(
				makesplit(100, "test", "v1", ""),
				makesplit(0.01, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
				require.Equal(t, float32(0.01), entry.Splits[1].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "3 splits",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v3", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
		},
		{
			name: "3 splits one duplicated same weights",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
			validateErr: "split destination occurs more than once",
		},
		{
			name: "3 splits one duplicated diff weights",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v1", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
			validateErr: "split destination occurs more than once",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceSplitMergeParent(t *testing.T) {

	type testCase struct {
		name                string
		split, parent, want *ServiceSplit
		wantErr             string
	}

	run := func(t *testing.T, tc testCase) {
		got, err := tc.split.MergeParent(tc.parent)
		if tc.wantErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		}
	}

	testCases := []testCase{
		{
			name: "all header manip fields set",
			split: &ServiceSplit{
				Weight:  50.0,
				Service: "foo",
				RequestHeaders: &HTTPHeaderModifiers{
					Add: map[string]string{
						"child-only":      "1",
						"both-want-child": "2",
					},
					Set: map[string]string{
						"child-only":      "3",
						"both-want-child": "4",
					},
					Remove: []string{"child-only-req", "both-req"},
				},
				ResponseHeaders: &HTTPHeaderModifiers{
					Add: map[string]string{
						"child-only":       "5",
						"both-want-parent": "6",
					},
					Set: map[string]string{
						"child-only":       "7",
						"both-want-parent": "8",
					},
					Remove: []string{"child-only-resp", "both-resp"},
				},
			},
			parent: &ServiceSplit{
				Weight:  25.0,
				Service: "bar",
				RequestHeaders: &HTTPHeaderModifiers{
					Add: map[string]string{
						"parent-only":     "9",
						"both-want-child": "10",
					},
					Set: map[string]string{
						"parent-only":     "11",
						"both-want-child": "12",
					},
					Remove: []string{"parent-only-req", "both-req"},
				},
				ResponseHeaders: &HTTPHeaderModifiers{
					Add: map[string]string{
						"parent-only":      "13",
						"both-want-parent": "14",
					},
					Set: map[string]string{
						"parent-only":      "15",
						"both-want-parent": "16",
					},
					Remove: []string{"parent-only-resp", "both-resp"},
				},
			},
			want: &ServiceSplit{
				Weight:  50.0,
				Service: "foo",
				RequestHeaders: &HTTPHeaderModifiers{
					Add: map[string]string{
						"child-only":      "1",
						"both-want-child": "2",
						"parent-only":     "9",
					},
					Set: map[string]string{
						"child-only":      "3",
						"both-want-child": "4",
						"parent-only":     "11",
					},
					Remove: []string{"parent-only-req", "both-req", "child-only-req"},
				},
				ResponseHeaders: &HTTPHeaderModifiers{
					Add: map[string]string{
						"child-only":       "5",
						"parent-only":      "13",
						"both-want-parent": "14",
					},
					Set: map[string]string{
						"child-only":       "7",
						"parent-only":      "15",
						"both-want-parent": "16",
					},
					Remove: []string{"child-only-resp", "both-resp", "parent-only-resp"},
				},
			},
		},
		{
			name: "no header manip",
			split: &ServiceSplit{
				Weight:  50,
				Service: "foo",
			},
			parent: &ServiceSplit{
				Weight:  50,
				Service: "bar",
			},
			want: &ServiceSplit{
				Weight:  50,
				Service: "foo",
			},
		},
		{
			name: "nil parent",
			split: &ServiceSplit{
				Weight:  50,
				Service: "foo",
			},
			parent: nil,
			want: &ServiceSplit{
				Weight:  50,
				Service: "foo",
			},
		},
		{
			name:  "nil child",
			split: nil,
			parent: &ServiceSplit{
				Weight:  50,
				Service: "foo",
			},
			want: &ServiceSplit{
				Weight:  50,
				Service: "foo",
			},
		},
		{
			name:   "both nil",
			split:  nil,
			parent: nil,
			want:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestServiceRouterConfigEntry(t *testing.T) {

	httpMatch := func(http *ServiceRouteHTTPMatch) *ServiceRouteMatch {
		return &ServiceRouteMatch{HTTP: http}
	}
	httpMatchHeader := func(headers ...ServiceRouteHTTPMatchHeader) *ServiceRouteMatch {
		return httpMatch(&ServiceRouteHTTPMatch{
			Header: headers,
		})
	}
	httpMatchParam := func(params ...ServiceRouteHTTPMatchQueryParam) *ServiceRouteMatch {
		return httpMatch(&ServiceRouteHTTPMatch{
			QueryParam: params,
		})
	}
	toService := func(svc string) *ServiceRouteDestination {
		return &ServiceRouteDestination{Service: svc}
	}
	routeMatch := func(match *ServiceRouteMatch) ServiceRoute {
		return ServiceRoute{
			Match:       match,
			Destination: toService("other"),
		}
	}
	makerouter := func(routes ...ServiceRoute) *ServiceRouterConfigEntry {
		return &ServiceRouterConfigEntry{
			Kind:   ServiceRouter,
			Name:   "test",
			Routes: routes,
		}
	}

	type testcase struct {
		name         string
		entry        *ServiceRouterConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceRouterConfigEntry)
	}

	cases := []testcase{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceRouterConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name:  "empty",
			entry: makerouter(),
		},
		{
			name: "1 empty route",
			entry: makerouter(
				ServiceRoute{},
			),
		},

		{
			name: "route with path exact",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact: "/exact",
			}))),
		},
		{
			name: "route with bad path exact",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact: "no-leading-slash",
			}))),
			validateErr: "PathExact doesn't start with '/'",
		},
		{
			name: "route with path prefix",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathPrefix: "/prefix",
			}))),
		},
		{
			name: "route with bad path prefix",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathPrefix: "no-leading-slash",
			}))),
			validateErr: "PathPrefix doesn't start with '/'",
		},
		{
			name: "route with path regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathRegex: "/regex",
			}))),
		},
		{
			name: "route with path exact and prefix",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact:  "/exact",
				PathPrefix: "/prefix",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},
		{
			name: "route with path exact and regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact: "/exact",
				PathRegex: "/regex",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},
		{
			name: "route with path prefix and regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathPrefix: "/prefix",
				PathRegex:  "/regex",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},
		{
			name: "route with path exact, prefix, and regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact:  "/exact",
				PathPrefix: "/prefix",
				PathRegex:  "/regex",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},

		{
			name: "route with no name header",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Present: true,
			}))),
			validateErr: "missing required Name field",
		},
		{
			name: "route with header present",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
			}))),
		},
		{
			name: "route with header not present",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Invert:  true,
			}))),
		},
		{
			name: "route with header exact",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:  "foo",
				Exact: "bar",
			}))),
		},
		{
			name: "route with header regex",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:  "foo",
				Regex: "bar",
			}))),
		},
		{
			name: "route with header prefix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:   "foo",
				Prefix: "bar",
			}))),
		},
		{
			name: "route with header suffix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:   "foo",
				Suffix: "bar",
			}))),
		},
		{
			name: "route with header present and exact",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Exact:   "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		{
			name: "route with header present and regex",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Regex:   "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		{
			name: "route with header present and prefix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Prefix:  "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		{
			name: "route with header present and suffix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Suffix:  "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		// NOTE: Some combinatoric cases for header operators (some 5 choose 2,
		// all 5 choose 3, all 5 choose 4, all 5 choose 5) are omitted from
		// testing.

		////////////////
		{
			name: "route with no name query param",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Exact: "foo",
			}))),
			validateErr: "missing required Name field",
		},
		{
			name: "route with query param exact match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:  "foo",
				Exact: "bar",
			}))),
		},
		{
			name: "route with query param regex match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:  "foo",
				Regex: "bar",
			}))),
		},
		{
			name: "route with query param present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Present: true,
			}))),
		},
		{
			name: "route with query param exact and regex match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:  "foo",
				Exact: "bar",
				Regex: "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		{
			name: "route with query param exact and present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Exact:   "bar",
				Present: true,
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		{
			name: "route with query param regex and present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Regex:   "bar",
				Present: true,
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		{
			name: "route with query param exact, regex, and present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Exact:   "bar",
				Regex:   "bar",
				Present: true,
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		////////////////
		{
			name: "route with no match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: nil,
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		{
			name: "route with path prefix match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatch(&ServiceRouteHTTPMatch{
					PathPrefix: "/api",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
		},
		{
			name: "route with path exact match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatch(&ServiceRouteHTTPMatch{
					PathExact: "/api",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
		},
		{
			name: "route with path regex match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatch(&ServiceRouteHTTPMatch{
					PathRegex: "/api",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		{
			name: "route with header match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatchHeader(ServiceRouteHTTPMatchHeader{
					Name:  "foo",
					Exact: "bar",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		{
			name: "route with header match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatchParam(ServiceRouteHTTPMatchQueryParam{
					Name:  "foo",
					Exact: "bar",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		////////////////
		{
			name: "route with method matches",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				Methods: []string{
					"get", "POST", "dElEtE",
				},
			}))),
			check: func(t *testing.T, entry *ServiceRouterConfigEntry) {
				m := entry.Routes[0].Match.HTTP.Methods
				require.Equal(t, []string{"GET", "POST", "DELETE"}, m)
			},
		},
		{
			name: "route with method matches repeated",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				Methods: []string{
					"GET", "DELETE", "get",
				},
			}))),
			validateErr: "Methods contains \"GET\" more than once",
		},
		////////////////
		{
			name: "route with no match with retry condition",
			entry: makerouter(ServiceRoute{
				Match: nil,
				Destination: &ServiceRouteDestination{
					Service: "other",
					RetryOn: []string{
						"5xx",
						"gateway-error",
						"reset",
						"connect-failure",
						"envoy-ratelimited",
						"retriable-4xx",
						"refused-stream",
						"cancelled",
						"deadline-exceeded",
						"internal",
						"resource-exhausted",
						"unavailable",
					},
				},
			}),
		},
		{
			name: "route with no match with invalid retry condition",
			entry: makerouter(ServiceRoute{
				Match: nil,
				Destination: &ServiceRouteDestination{
					Service: "other",
					RetryOn: []string{
						"invalid-retry-condition",
					},
				},
			}),
			validateErr: "contains an invalid retry condition: \"invalid-retry-condition\"",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

var validSubsetNames = []string{
	"a", "aa", "2a", "a2", "a2a", "a22a",
	"1", "11", "10", "01",
	"a-a", "a--a", "a--a--a",
	"0-0", "0--0", "0--0--0",
	strings.Repeat("a", 63),
}

var invalidSubsetNames = []string{
	"A", "AA", "2A", "A2", "A2A", "A22A",
	"A-A", "A--A", "A--A--A",
	" ", " a", "a ", "a a",
	"_", "_a", "a_", "a_a",
	".", ".a", "a.", "a.a",
	"-", "-a", "a-",
	strings.Repeat("a", 64),
}

func TestValidateServiceSubset(t *testing.T) {
	for _, name := range validSubsetNames {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, validateServiceSubset(name))
		})
	}

	for _, name := range invalidSubsetNames {
		t.Run(name, func(t *testing.T) {
			require.Error(t, validateServiceSubset(name))
		})
	}
}

func TestIsProtocolHTTPLike(t *testing.T) {
	assert.False(t, IsProtocolHTTPLike(""))
	assert.False(t, IsProtocolHTTPLike("tcp"))

	assert.True(t, IsProtocolHTTPLike("http"))
	assert.True(t, IsProtocolHTTPLike("http2"))
	assert.True(t, IsProtocolHTTPLike("grpc"))
}

func TestIsValidRetryCondition(t *testing.T) {
	assert.False(t, isValidRetryCondition(""))
	assert.False(t, isValidRetryCondition("retriable-headers"))
	assert.False(t, isValidRetryCondition("retriable-status-codes"))

	assert.True(t, isValidRetryCondition("5xx"))
	assert.True(t, isValidRetryCondition("gateway-error"))
	assert.True(t, isValidRetryCondition("reset"))
	assert.True(t, isValidRetryCondition("connect-failure"))
	assert.True(t, isValidRetryCondition("envoy-ratelimited"))
	assert.True(t, isValidRetryCondition("retriable-4xx"))
	assert.True(t, isValidRetryCondition("refused-stream"))
	assert.True(t, isValidRetryCondition("cancelled"))
	assert.True(t, isValidRetryCondition("deadline-exceeded"))
	assert.True(t, isValidRetryCondition("internal"))
	assert.True(t, isValidRetryCondition("resource-exhausted"))
	assert.True(t, isValidRetryCondition("unavailable"))
}
