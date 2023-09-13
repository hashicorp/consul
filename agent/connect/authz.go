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
	target, targetNS, targetAP string,
	ixn *structs.Intention,
	matchType structs.IntentionMatchType,
) (auth bool, match bool) {

	switch matchType {
	case structs.IntentionMatchDestination:
		if acl.PartitionOrDefault(ixn.DestinationPartition) != acl.PartitionOrDefault(targetAP) {
			return false, false
		}

		if ixn.DestinationNS != structs.WildcardSpecifier && ixn.DestinationNS != targetNS {
			// Non-matching namespace
			return false, false
		}

		if ixn.DestinationName != structs.WildcardSpecifier && ixn.DestinationName != target {
			// Non-matching name
			return false, false
		}

	case structs.IntentionMatchSource:
		if acl.PartitionOrDefault(ixn.SourcePartition) != acl.PartitionOrDefault(targetAP) {
			return false, false
		}

		if ixn.SourceNS != structs.WildcardSpecifier && ixn.SourceNS != targetNS {
			// Non-matching namespace
			return false, false
		}

		if ixn.SourceName != structs.WildcardSpecifier && ixn.SourceName != target {
			// Non-matching name
			return false, false
		}

	default:
		// Reject on any un-recognized match type
		return false, false
	}

	// The name and namespace match, so the destination is covered
	return ixn.Action == structs.IntentionActionAllow, true
}
