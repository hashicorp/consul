// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

// parsedLabels defines valid DNS labels that are possible for ALL DNS query in Consul. (v1 and v2, CE and ENT)
// It is the job of the parser to populate the struct, the routers to call the query processor,
// and the query processor to validate is the labels.
type parsedLabels struct {
	Datacenter       string
	Namespace        string
	Partition        string
	Peer             string
	PeerOrDatacenter string // deprecated: use Datacenter or Peer
	SamenessGroup    string
}

// ParseLabels can parse a DNS query's labels and returns a parsedLabels.
// It also does light validation according to invariants across all possible DNS queries for all Consul versions
func parseLabels(labels []string) (*parsedLabels, bool) {
	var result parsedLabels

	switch len(labels) {
	case 2, 4, 6:
		// Supports the following formats:
		// - [.<namespace>.ns][.<partition>.ap][.<datacenter>.dc]
		// - <namespace>.<datacenter>
		// - [.<namespace>.ns][.<partition>.ap][.<peer>.peer]
		// - [.<samenessGroup>.sg][.<partition>.ap][.<namespace>.ns]
		for i := 0; i < len(labels); i += 2 {
			switch labels[i+1] {
			case "ns":
				result.Namespace = labels[i]
			case "ap":
				result.Partition = labels[i]
			case "dc", "cluster":
				result.Datacenter = labels[i]
			case "sg":
				result.SamenessGroup = labels[i]
			case "peer":
				result.Peer = labels[i]
			default:
				// The only case in which labels[i+1] is allowed to be a value
				// other than ns, ap, or dc is if n == 2 to support the format:
				// <namespace>.<datacenter>.
				if len(labels) == 2 {
					result.PeerOrDatacenter = labels[1]
					result.Namespace = labels[0]
					return &result, true
				}
				return nil, false
			}
		}

		// VALIDATIONS
		// Return nil result and false boolean when both datacenter and peer are specified.
		if result.Datacenter != "" && result.Peer != "" {
			return nil, false
		}

		// Validate that this a valid DNS including sg
		if result.SamenessGroup != "" && (result.Datacenter != "" || result.Peer != "") {
			return nil, false
		}

		return &result, true

	case 1:
		result.PeerOrDatacenter = labels[0]
		return &result, true

	case 0:
		return &result, true
	}

	return &result, false
}

// parsePort looks through the query parts for a named port label.
// It assumes the only valid input format is["<portName>", "port", "<targetName>"].
// The other expected formats are ["<targetName>"] and ["<tag>", "<targetName>"].
// It is expected that the queryProcessor validates if the label is allowed for the query type.
func parsePort(parts []string) string {
	// The minimum number of parts would be
	if len(parts) != 3 || parts[1] != "port" {
		return ""
	}
	return parts[0]
}
