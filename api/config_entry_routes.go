package api

// TCPRouteConfigEntry -- TODO stub
type TCPRouteConfigEntry struct {
	// Kind of the config entry. This should be set to api.TCPRoute.
	Kind string

	// Name is used to match the config entry with its associated tcp-route
	// service. This should match the name provided in the service definition.
	Name string

	// Parents is a list of gateways that this route should be bound to.
	Parents []ResourceReference
	// Services is a list of TCP-based services that this should route to.
	// Currently, this must specify at maximum one service.
	Services []TCPService

	Meta map[string]string `json:",omitempty"`

	// Status is the asynchronous status which a TCPRoute propagates to the user.
	Status ConfigEntryStatus

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

// TCPService is a service reference for a TCPRoute
type TCPService struct {
	Name string
	// Weight specifies the proportion of requests forwarded to the referenced service.
	// This is computed as weight/(sum of all weights in the list of services).
	Weight int

	// Partition is the partition the config entry is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the config entry is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`
}

// HTTPRouteConfigEntry manages the configuration for a HTTP route
// with the given name.
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
