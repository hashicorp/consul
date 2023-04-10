// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

import "fmt"

func (s *SamenessGroupConfigEntry) Validate() error {
	return fmt.Errorf("sameness-groups are an enterprise-only feature")
}

// RelatedPeers returns all peers that are members of a sameness group config entry.
func (s *SamenessGroupConfigEntry) RelatedPeers() []string {
	return nil
}

// AllMembers adds the local partition to Members when it is set.
func (s *SamenessGroupConfigEntry) AllMembers() []SamenessGroupMember {
	return nil
}

func (s *SamenessGroupConfigEntry) ToFailoverTargets() []ServiceResolverFailoverTarget {
	return nil
}
