// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

import "fmt"

// Validate assures that the sameness-groups are an enterprise only feature
func (s *SamenessGroupConfigEntry) Validate() error {
	return fmt.Errorf("sameness-groups are an enterprise-only feature")
}

// RelatedPeers is an OSS placeholder noop
func (s *SamenessGroupConfigEntry) RelatedPeers() []string {
	return nil
}

// AllMembers is an OSS placeholder noop
func (s *SamenessGroupConfigEntry) AllMembers() []SamenessGroupMember {
	return nil
}

// ToServiceResolverFailoverTargets is an OSS placeholder noop
func (s *SamenessGroupConfigEntry) ToServiceResolverFailoverTargets() []ServiceResolverFailoverTarget {
	return nil
}

// ToQueryFailoverTargets is an OSS placeholder noop
func (s *SamenessGroupConfigEntry) ToQueryFailoverTargets(namespace string) []QueryFailoverTarget {
	return nil
}
