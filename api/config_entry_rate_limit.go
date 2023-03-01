package api

type readWriteRatesConfig struct {
	ReadRate  float32
	WriteRate float32
}

type RateLimitConfigEntry struct {
	// Kind of the config entry. This will be set to structs.RateLimitConfig
	Kind string
	// what is the name used for
	Name string
	Mode string // {permissive, enforcing, disabled}

	// what is this
	Meta map[string]string `json:",omitempty"`
	// overall limits
	ReadRate  float32
	WriteRate float32

	//limits specific to a type of call
	ACL            readWriteRatesConfig `json:",omitempty"`
	Catalog        readWriteRatesConfig `json:",omitempty"`
	ConfigEntry    readWriteRatesConfig `json:",omitempty"`
	ConnectCA      readWriteRatesConfig `json:",omitempty"`
	Coordinate     readWriteRatesConfig `json:",omitempty"`
	DiscoveryChain readWriteRatesConfig `json:",omitempty"`
	Health         readWriteRatesConfig `json:",omitempty"`
	Intention      readWriteRatesConfig `json:",omitempty"`
	KV             readWriteRatesConfig `json:",omitempty"`
	Tenancy        readWriteRatesConfig `json:",omitempty"`
	PreparedQuery  readWriteRatesConfig `json:",omitempty"`
	Session        readWriteRatesConfig `json:",omitempty"`
	Txn            readWriteRatesConfig `json:",omitempty"`

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

func (r *RateLimitConfigEntry) GetKind() string {
	return RateLimitConfig
}
func (r *RateLimitConfigEntry) GetName() string {
	if r == nil {
		return ""
	}
	return r.Name
}
func (r *RateLimitConfigEntry) GetPartition() string {
	return r.Partition
}
func (r *RateLimitConfigEntry) GetNamespace() string {
	return r.Namespace
}
func (r *RateLimitConfigEntry) GetMeta() map[string]string {
	if r == nil {
		return nil
	}
	return r.Meta
}
func (r *RateLimitConfigEntry) GetCreateIndex() uint64 {
	return r.CreateIndex
}
func (r *RateLimitConfigEntry) GetModifyIndex() uint64 {
	return r.ModifyIndex
}
