// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package meshv1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
)

type routeWithAddons interface {
	proto.Message
	GetUnderlyingBackendRefs() []*BackendReference
}

func TestXRoute_GetUnderlyingBackendRefs(t *testing.T) {
	type testcase struct {
		route  routeWithAddons
		expect []*BackendReference
	}

	run := func(t *testing.T, tc testcase) {
		got := tc.route.GetUnderlyingBackendRefs()
		require.ElementsMatch(t, stringifyList(tc.expect), stringifyList(got))
	}

	cases := map[string]testcase{
		"http: nil": {
			route: (*HTTPRoute)(nil),
		},
		"grpc: nil": {
			route: (*GRPCRoute)(nil),
		},
		"tcp: nil": {
			route: (*TCPRoute)(nil),
		},
		"http: kitchen sink": {
			route: &HTTPRoute{
				Rules: []*HTTPRouteRule{
					{BackendRefs: []*HTTPBackendRef{
						{BackendRef: newBackendRef("aa")},
					}},
					{BackendRefs: []*HTTPBackendRef{
						{BackendRef: newBackendRef("bb")},
					}},
					{BackendRefs: []*HTTPBackendRef{
						{BackendRef: newBackendRef("cc")},
						{BackendRef: newBackendRef("dd")},
					}},
					{BackendRefs: []*HTTPBackendRef{
						{BackendRef: newBackendRef("ee")},
						{BackendRef: newBackendRef("ff")},
					}},
				},
			},
			expect: []*BackendReference{
				newBackendRef("aa"),
				newBackendRef("bb"),
				newBackendRef("cc"),
				newBackendRef("dd"),
				newBackendRef("ee"),
				newBackendRef("ff"),
			},
		},
		"grpc: kitchen sink": {
			route: &GRPCRoute{
				Rules: []*GRPCRouteRule{
					{BackendRefs: []*GRPCBackendRef{
						{BackendRef: newBackendRef("aa")},
					}},
					{BackendRefs: []*GRPCBackendRef{
						{BackendRef: newBackendRef("bb")},
					}},
					{BackendRefs: []*GRPCBackendRef{
						{BackendRef: newBackendRef("cc")},
						{BackendRef: newBackendRef("dd")},
					}},
					{BackendRefs: []*GRPCBackendRef{
						{BackendRef: newBackendRef("ee")},
						{BackendRef: newBackendRef("ff")},
					}},
				},
			},
			expect: []*BackendReference{
				newBackendRef("aa"),
				newBackendRef("bb"),
				newBackendRef("cc"),
				newBackendRef("dd"),
				newBackendRef("ee"),
				newBackendRef("ff"),
			},
		},
		"tcp: kitchen sink": {
			route: &TCPRoute{
				Rules: []*TCPRouteRule{
					{BackendRefs: []*TCPBackendRef{
						{BackendRef: newBackendRef("aa")},
					}},
					{BackendRefs: []*TCPBackendRef{
						{BackendRef: newBackendRef("bb")},
					}},
					{BackendRefs: []*TCPBackendRef{
						{BackendRef: newBackendRef("cc")},
						{BackendRef: newBackendRef("dd")},
					}},
					{BackendRefs: []*TCPBackendRef{
						{BackendRef: newBackendRef("ee")},
						{BackendRef: newBackendRef("ff")},
					}},
				},
			},
			expect: []*BackendReference{
				newBackendRef("aa"),
				newBackendRef("bb"),
				newBackendRef("cc"),
				newBackendRef("dd"),
				newBackendRef("ee"),
				newBackendRef("ff"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func protoToString[V proto.Message](pb V) string {
	m := protojson.MarshalOptions{
		Indent: "  ",
	}
	gotJSON, err := m.Marshal(pb)
	if err != nil {
		return "<ERR: " + err.Error() + ">"
	}
	return string(gotJSON)
}

func newRouteRef(name string) *pbresource.Reference {
	return &pbresource.Reference{
		Type: &pbresource.Type{
			Group:        "fake",
			GroupVersion: "v1alpha1",
			Kind:         "fake",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		},
		Name: name,
	}
}

func newBackendRef(name string) *BackendReference {
	return &BackendReference{
		Ref: newRouteRef(name),
	}
}

func stringifyList[V proto.Message](list []V) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, protoToString(item))
	}
	return out
}
