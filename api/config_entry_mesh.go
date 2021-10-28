package api

import "encoding/json"

// MeshConfigEntry manages the global configuration for all service mesh
// proxies.
type MeshConfigEntry struct {
	// Partition is the partition the MeshConfigEntry applies to.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the MeshConfigEntry applies to.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	// TransparentProxy applies configuration specific to proxies
	// in transparent mode.
	TransparentProxy TransparentProxyMeshConfig `alias:"transparent_proxy"`

	Meta map[string]string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64
}

type TransparentProxyMeshConfig struct {
	MeshDestinationsOnly bool `alias:"mesh_destinations_only"`
}

func (e *MeshConfigEntry) GetKind() string            { return MeshConfig }
func (e *MeshConfigEntry) GetName() string            { return MeshConfigMesh }
func (e *MeshConfigEntry) GetPartition() string       { return e.Partition }
func (e *MeshConfigEntry) GetNamespace() string       { return e.Namespace }
func (e *MeshConfigEntry) GetMeta() map[string]string { return e.Meta }
func (e *MeshConfigEntry) GetCreateIndex() uint64     { return e.CreateIndex }
func (e *MeshConfigEntry) GetModifyIndex() uint64     { return e.ModifyIndex }

// MarshalJSON adds the Kind field so that the JSON can be decoded back into the
// correct type.
func (e *MeshConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias MeshConfigEntry
	source := &struct {
		Kind string
		*Alias
	}{
		Kind:  MeshConfig,
		Alias: (*Alias)(e),
	}
	return json.Marshal(source)
}
