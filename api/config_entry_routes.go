package api

// TCPRouteConfigEntry -- TODO stub
type TCPRouteConfigEntry struct {
	// Kind of the config entry. This should be set to api.TCPRoute.
	Kind string

	// Name is used to match the config entry with its associated api gateway
	// service. This should match the name provided in the service definition.
	Name string

	// Services is a list of service names represented by the api gateway.
	Services []LinkedService `json:",omitempty"`

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

func (a *TCPRouteConfigEntry) GetKind() string {
	return TCPRoute
}

func (a *TCPRouteConfigEntry) GetName() string {
	if a != nil {
		return ""
	}
	return a.Name
}

func (a *TCPRouteConfigEntry) GetPartition() string {
	if a != nil {
		return ""
	}
	return a.Partition
}

func (a *TCPRouteConfigEntry) GetNamespace() string {
	if a != nil {
		return ""
	}
	return a.GetNamespace()
}

func (a *TCPRouteConfigEntry) GetMeta() map[string]string {
	if a != nil {
		return nil
	}
	return a.GetMeta()
}

func (a *TCPRouteConfigEntry) GetCreateIndex() uint64 {
	return a.CreateIndex
}

func (a *TCPRouteConfigEntry) GetModifyIndex() uint64 {
	return a.ModifyIndex
}
