// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

type ReadWriteRatesConfig struct {
	ReadRate  float64
	WriteRate float64
}

type RateLimitIPConfigEntry struct {
	// Kind of the config entry. This will be set to structs.RateLimitIPConfig
	Kind string
	Name string
	Mode string // {permissive, enforcing, disabled}

	Meta map[string]string `json:",omitempty"`
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
}

func (r *RateLimitIPConfigEntry) GetKind() string {
	return RateLimitIPConfig
}
func (r *RateLimitIPConfigEntry) GetName() string {
	if r == nil {
		return ""
	}
	return r.Name
}
func (r *RateLimitIPConfigEntry) GetPartition() string {
	return r.Partition
}
func (r *RateLimitIPConfigEntry) GetNamespace() string {
	return r.Namespace
}
func (r *RateLimitIPConfigEntry) GetMeta() map[string]string {
	if r == nil {
		return nil
	}
	return r.Meta
}
func (r *RateLimitIPConfigEntry) GetCreateIndex() uint64 {
	return r.CreateIndex
}
func (r *RateLimitIPConfigEntry) GetModifyIndex() uint64 {
	return r.ModifyIndex
}
