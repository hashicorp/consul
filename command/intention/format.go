// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package intention

import (
	"github.com/hashicorp/consul/api"
)

// FormatSource returns the namespace/name format for the source. This is
// different from (*api.Intention).SourceString in that the default namespace
// is not omitted.
func FormatSource(i *api.Intention) string {
	return partString(i.SourceNS, i.SourceName)
}

// FormatDestination returns the namespace/name format for the destination.
// This is different from (*api.Intention).DestinationString in that the
// default namespace is not omitted.
func FormatDestination(i *api.Intention) string {
	return partString(i.DestinationNS, i.DestinationName)
}

func partString(ns, n string) string {
	if ns == "" {
		return n
	}
	return ns + "/" + n
}
