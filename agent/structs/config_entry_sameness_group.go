// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/consul/acl"
)

type SamenessGroupConfigEntry struct {
	Name               string
	DefaultForFailover bool `json:",omitempty" alias:"default_for_failover"`
	IncludeLocal       bool `json:",omitempty" alias:"include_local"`
	Members            []SamenessGroupMember
	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (s *SamenessGroupConfigEntry) GetKind() string            { return SamenessGroup }
func (s *SamenessGroupConfigEntry) GetName() string            { return s.Name }
func (s *SamenessGroupConfigEntry) GetMeta() map[string]string { return s.Meta }
func (s *SamenessGroupConfigEntry) GetCreateIndex() uint64     { return s.CreateIndex }
func (s *SamenessGroupConfigEntry) GetModifyIndex() uint64     { return s.ModifyIndex }

func (s *SamenessGroupConfigEntry) GetRaftIndex() *RaftIndex {
	if s == nil {
		return &RaftIndex{}
	}
	return &s.RaftIndex
}

func (s *SamenessGroupConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if s == nil {
		return nil
	}
	return &s.EnterpriseMeta
}

func (s *SamenessGroupConfigEntry) Normalize() error {
	if s == nil {
		return fmt.Errorf("config entry is nil")
	}
	s.EnterpriseMeta.Normalize()
	return nil
}

func (s *SamenessGroupConfigEntry) CanRead(authz acl.Authorizer) error {
	return nil
}

func (s *SamenessGroupConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	s.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (s *SamenessGroupConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias SamenessGroupConfigEntry
	source := &struct {
		Kind string
		*Alias
	}{
		Kind:  SamenessGroup,
		Alias: (*Alias)(s),
	}
	return json.Marshal(source)
}

type SamenessGroupMember struct {
	Partition string `json:",omitempty"`
	Peer      string `json:",omitempty"`
}
