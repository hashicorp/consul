package api

type HTTPRouteConfigEntry struct {
	// Kind of the config entry. This should be set to api.HTTPRoute.
	Kind string

	// Name is used to match the config entry with its associated http-route.
	Name string

	Meta map[string]string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64

	// Partition is the partition the config entry is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the config entry is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`
}

func (r *HTTPRouteConfigEntry) GetKind() string            { return HTTPRoute }
func (r *HTTPRouteConfigEntry) GetName() string            { return r.Name }
func (r *HTTPRouteConfigEntry) GetPartition() string       { return r.Partition }
func (r *HTTPRouteConfigEntry) GetNamespace() string       { return r.Namespace }
func (r *HTTPRouteConfigEntry) GetMeta() map[string]string { return r.Meta }
func (r *HTTPRouteConfigEntry) GetCreateIndex() uint64     { return r.CreateIndex }
func (r *HTTPRouteConfigEntry) GetModifyIndex() uint64     { return r.ModifyIndex }
