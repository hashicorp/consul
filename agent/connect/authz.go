// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// AuthorizeIntentionTarget determines whether the destination is covered by the given intention
// and whether the intention action allows a connection.
// This is a generalized version of the old CertURI.Authorize(), and can be evaluated against sources or destinations.
//
// The return value of `auth` is only valid if the second value `match` is true.
// If `match` is false, then the intention doesn't match this target and any result should be ignored.
func AuthorizeIntentionTarget(
	target, targetNS, targetAP, targetPeer string,
	ixn *structs.Intention,
	matchType structs.IntentionMatchType,
) (bool, bool) {

	match := IntentionMatch(target, targetNS, targetAP, targetPeer, ixn, matchType)

	if match {
		return ixn.Action == structs.IntentionActionAllow, true
	} else {
		return false, false
	}
}

// IntentionMatch determines whether the target is covered by the given intention.
func IntentionMatch(
	target, targetNS, targetAP, targetPeer string,
	ixn *structs.Intention,
	matchType structs.IntentionMatchType,
) bool {

	switch matchType {
	case structs.IntentionMatchDestination:
		if acl.PartitionOrDefault(ixn.DestinationPartition) != acl.PartitionOrDefault(targetAP) {
			return false
		}

		if ixn.DestinationNS != structs.WildcardSpecifier && ixn.DestinationNS != targetNS {
			// Non-matching namespace
			return false
		}

		if ixn.DestinationName != structs.WildcardSpecifier && ixn.DestinationName != target {
			// Non-matching name
			return false
		}

	case structs.IntentionMatchSource:
		if ixn.SourcePeer != targetPeer {
			return false
		}

		if acl.PartitionOrDefault(ixn.SourcePartition) != acl.PartitionOrDefault(targetAP) {
			return false
		}

		if ixn.SourceNS != structs.WildcardSpecifier && ixn.SourceNS != targetNS {
			// Non-matching namespace
			return false
		}

		if ixn.SourceName != structs.WildcardSpecifier && ixn.SourceName != target {
			// Non-matching name
			return false
		}

	default:
		// Reject on any un-recognized match type
		return false
	}

	// The name and namespace match, so the destination is covered
	return true
}
