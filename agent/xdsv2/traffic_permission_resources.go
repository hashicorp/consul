// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"

	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

func makeL4Intentions() {
	// need to take WI CTFs and translate to service intentions
}

func makeRBACNetworkFilter(l4 *pbproxystate.L4Destination) (*envoy_listener_v3.Filter, error) {

	// TODO: temporary use of old logic
	//	rbacNetworkFilter := xds.makeRBACNetworkFilter(
	//		intentions structs.SimplifiedIntentions,
	//		intentionDefaultAllow bool,
	//		localInfo rbacLocalInfo,
	//		peerTrustBundles []*pbpeering.PeeringTrustBundle,
	//)

	rbacNetworkFilter, err := makeEmptyRBACNetworkFilter()
	return rbacNetworkFilter, err
}
