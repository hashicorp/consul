// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestIntention returns a valid, uninserted (no ID set) intention.
func TestIntention(t testing.T) *Intention {
	ixn := &Intention{
		SourceNS:        IntentionDefaultNamespace,
		SourceName:      "api",
		DestinationNS:   IntentionDefaultNamespace,
		DestinationName: "db",
		Action:          IntentionActionAllow,
		SourceType:      IntentionSourceConsul,
		Meta:            map[string]string{},
	}
	ixn.FillPartitionAndNamespace(nil, true)
	return ixn
}
