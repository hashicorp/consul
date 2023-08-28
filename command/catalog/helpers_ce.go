// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package catalog

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

func NodesHeader(isDetailed bool) string {
	if isDetailed {
		return "Node\x1fID\x1fAddress\x1fDC\x1fTaggedAddresses\x1fMeta"
	} else {
		return "Node\x1fID\x1fAddress\x1fDC"
	}
}

func NodeRow(node *api.Node, isDetailed bool) string {
	if isDetailed {
		return fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s",
			node.Node, node.ID, node.Address, node.Datacenter,
			mapToKV(node.TaggedAddresses, ", "), mapToKV(node.Meta, ", "))
	} else {
		// Shorten the ID in non-detailed mode to just the first octet.
		id := node.ID
		idx := strings.Index(id, "-")
		if idx > 0 {
			id = id[0:idx]
		}
		return fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s",
			node.Node, id, node.Address, node.Datacenter)
	}
}
