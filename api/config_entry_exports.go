package api

import "encoding/json"

// PartitionExportsConfigEntry manages the exported services for a single admin partition.
// Admin Partitions are a Consul Enterprise feature.
type PartitionExportsConfigEntry struct {
	// Name is the name of the partition the PartitionExportsConfigEntry applies to.
	// Partitioning is a Consul Enterprise feature.
	Name string `json:",omitempty"`

	// Partition is the partition where the PartitionExportsConfigEntry is stored.
	// If the partition does not match the name, the name will overwrite the partition.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Services is a list of services to be exported and the list of partitions
	// to expose them to.
	Services []ExportedService

	Meta map[string]string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64
}

// ExportedService manages the exporting of a service in the local partition to
// other partitions.
type ExportedService struct {
	// Name is the name of the service to be exported.
	Name string

	// Namespace is the namespace to export the service from.
	Namespace string `json:",omitempty"`

	// Consumers is a list of downstream consumers of the service to be exported.
	Consumers []ServiceConsumer
}

// ServiceConsumer represents a downstream consumer of the service to be exported.
type ServiceConsumer struct {
	// Partition is the admin partition to export the service to.
	Partition string
}

func (e *PartitionExportsConfigEntry) GetKind() string            { return PartitionExports }
func (e *PartitionExportsConfigEntry) GetName() string            { return e.Name }
func (e *PartitionExportsConfigEntry) GetPartition() string       { return e.Name }
func (e *PartitionExportsConfigEntry) GetNamespace() string       { return IntentionDefaultNamespace }
func (e *PartitionExportsConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *PartitionExportsConfigEntry) GetCreateIndex() uint64     { return e.CreateIndex }
func (e *PartitionExportsConfigEntry) GetModifyIndex() uint64     { return e.ModifyIndex }

// MarshalJSON adds the Kind field so that the JSON can be decoded back into the
// correct type.
func (e *PartitionExportsConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias PartitionExportsConfigEntry
	source := &struct {
		Kind string
		*Alias
	}{
		Kind:  PartitionExports,
		Alias: (*Alias)(e),
	}
	return json.Marshal(source)
}
