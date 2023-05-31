// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/consul/acl"
)

type ReadWriteRatesConfig struct {
	ReadRate  float64
	WriteRate float64
}

type RateLimitIPConfigEntry struct {
	// Kind is the kind of configuration entry and must be "jwt-provider".
	Kind string `json:",omitempty"`

	// Name is the name of the provider being configured.
	Name               string            `json:",omitempty"`
	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	Mode               string // {permissive, enforcing, disabled}

	// overall limits
	ReadRate  float64
	WriteRate float64
	//limits specific to a type of call
	ACL            *ReadWriteRatesConfig `json:",omitempty"`
	Catalog        *ReadWriteRatesConfig `json:",omitempty"`
	ConfigEntry    *ReadWriteRatesConfig `json:",omitempty"`
	ConnectCA      *ReadWriteRatesConfig `json:",omitempty"`
	Coordinate     *ReadWriteRatesConfig `json:",omitempty"`
	DiscoveryChain *ReadWriteRatesConfig `json:",omitempty"`
	Health         *ReadWriteRatesConfig `json:",omitempty"`
	Intention      *ReadWriteRatesConfig `json:",omitempty"`
	KV             *ReadWriteRatesConfig `json:",omitempty"`
	Tenancy        *ReadWriteRatesConfig `json:",omitempty"`
	PreparedQuery  *ReadWriteRatesConfig `json:",omitempty"`
	Session        *ReadWriteRatesConfig `json:",omitempty"`
	Txn            *ReadWriteRatesConfig `json:",omitempty"`

	// Partition is the partition the config entry is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the config entry is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64
	RaftIndex
}

func (r *RateLimitIPConfigEntry) GetKind() string            { return RateLimitIPConfig }
func (r *RateLimitIPConfigEntry) GetName() string            { return r.Name }
func (r *RateLimitIPConfigEntry) GetMeta() map[string]string { return r.Meta }
func (r *RateLimitIPConfigEntry) GetCreateIndex() uint64     { return r.CreateIndex }
func (r *RateLimitIPConfigEntry) GetModifyIndex() uint64     { return r.ModifyIndex }

func (r *RateLimitIPConfigEntry) GetRaftIndex() *RaftIndex {
	if r == nil {
		return &RaftIndex{}
	}
	return &r.RaftIndex
}

func (r *RateLimitIPConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if r == nil {
		return nil
	}
	return &r.EnterpriseMeta
}

func (r *RateLimitIPConfigEntry) Normalize() error {
	if r == nil {
		return fmt.Errorf("config entry is nil")
	}
	r.EnterpriseMeta.Normalize()
	return nil
}

func (r *RateLimitIPConfigEntry) CanRead(authz acl.Authorizer) error {
	return nil
}

func (r *RateLimitIPConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	r.FillAuthzContext(&authzContext)
	// TODO: Implement
	// return authz.ToAllowAuthorizer().RateLimitIPAllowed(&authzContext)
	return nil
}

func (r *RateLimitIPConfigEntry) Validate() error {
	return nil
}

func (r *RateLimitIPConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias RateLimitIPConfigEntry
	source := &struct {
		Kind string
		*Alias
	}{
		Kind:  RateLimitIPConfig,
		Alias: (*Alias)(r),
	}
	return json.Marshal(source)
}
