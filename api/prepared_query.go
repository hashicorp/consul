package api

// QueryFailoverOptions sets options about how we fail over if there are no
// healthy nodes in the local datacenter.
type QueryFailoverOptions struct {
	// NearestN is set to the number of remote datacenters to try, based on
	// network coordinates.
	NearestN int

	// Datacenters is a fixed list of datacenters to try after NearestN. We
	// never try a datacenter multiple times, so those are subtracted from
	// this list before proceeding.
	Datacenters []string

	// Targets is a fixed list of datacenters and peers to try. This field cannot
	// be populated with NearestN or Datacenters.
	Targets []QueryFailoverTarget
}

// Deprecated: use QueryFailoverOptions instead.
type QueryDatacenterOptions = QueryFailoverOptions

type QueryFailoverTarget struct {
	// Peer specifies a peer to try during failover.
	Peer string

	// Datacenter specifies a datacenter to try during failover.
	Datacenter string

	// Partition specifies a partition to try during failover
	// Note: Partition are available only in Consul Enterprise
	Partition string

	// Namespace specifies a namespace to try during failover
	// Note: Namespaces are available only in Consul Enterprise
	Namespace string
}

// QueryDNSOptions controls settings when query results are served over DNS.
type QueryDNSOptions struct {
	// TTL is the time to live for the served DNS results.
	TTL string
}

// ServiceQuery is used to query for a set of healthy nodes offering a specific
// service.
type ServiceQuery struct {
	// Service is the service to query.
	Service string

	// Namespace of the service to query
	Namespace string `json:",omitempty"`

	// Near allows baking in the name of a node to automatically distance-
	// sort from. The magic "_agent" value is supported, which sorts near
	// the agent which initiated the request by default.
	Near string

	// Failover controls what we do if there are no healthy nodes in the
	// local datacenter.
	Failover QueryFailoverOptions

	// IgnoreCheckIDs is an optional list of health check IDs to ignore when
	// considering which nodes are healthy. It is useful as an emergency measure
	// to temporarily override some health check that is producing false negatives
	// for example.
	IgnoreCheckIDs []string

	// If OnlyPassing is true then we will only include nodes with passing
	// health checks (critical AND warning checks will cause a node to be
	// discarded)
	OnlyPassing bool

	// Tags are a set of required and/or disallowed tags. If a tag is in
	// this list it must be present. If the tag is preceded with "!" then
	// it is disallowed.
	Tags []string

	// NodeMeta is a map of required node metadata fields. If a key/value
	// pair is in this map it must be present on the node in order for the
	// service entry to be returned.
	NodeMeta map[string]string

	// ServiceMeta is a map of required service metadata fields. If a key/value
	// pair is in this map it must be present on the node in order for the
	// service entry to be returned.
	ServiceMeta map[string]string

	// Connect if true will filter the prepared query results to only
	// include Connect-capable services. These include both native services
	// and proxies for matching services. Note that if a proxy matches,
	// the constraints in the query above (Near, OnlyPassing, etc.) apply
	// to the _proxy_ and not the service being proxied. In practice, proxies
	// should be directly next to their services so this isn't an issue.
	Connect bool
}

// QueryTemplate carries the arguments for creating a templated query.
type QueryTemplate struct {
	// Type specifies the type of the query template. Currently only
	// "name_prefix_match" is supported. This field is required.
	Type string

	// Regexp allows specifying a regex pattern to match against the name
	// of the query being executed.
	Regexp string

	// RemoveEmptyTags if set to true, will cause the Tags list inside
	// the Service structure to be stripped of any empty strings. This is useful
	// when interpolating into tags in a way where the tag is optional, and
	// where searching for an empty tag would yield no results from the query.
	RemoveEmptyTags bool
}

// PreparedQueryDefinition defines a complete prepared query.
type PreparedQueryDefinition struct {
	// ID is this UUID-based ID for the query, always generated by Consul.
	ID string

	// Name is an optional friendly name for the query supplied by the
	// user. NOTE - if this feature is used then it will reduce the security
	// of any read ACL associated with this query/service since this name
	// can be used to locate nodes with supplying any ACL.
	Name string

	// Session is an optional session to tie this query's lifetime to. If
	// this is omitted then the query will not expire.
	Session string

	// Token is the ACL token used when the query was created, and it is
	// used when a query is subsequently executed. This token, or a token
	// with management privileges, must be used to change the query later.
	Token string

	// Service defines a service query (leaving things open for other types
	// later).
	Service ServiceQuery

	// DNS has options that control how the results of this query are
	// served over DNS.
	DNS QueryDNSOptions

	// Template is used to pass through the arguments for creating a
	// prepared query with an attached template. If a template is given,
	// interpolations are possible in other struct fields.
	Template QueryTemplate
}

// PreparedQueryExecuteResponse has the results of executing a query.
type PreparedQueryExecuteResponse struct {
	// Service is the service that was queried.
	Service string

	// Namespace of the service that was queried
	Namespace string `json:",omitempty"`

	// Nodes has the nodes that were output by the query.
	Nodes []ServiceEntry

	// DNS has the options for serving these results over DNS.
	DNS QueryDNSOptions

	// Datacenter is the datacenter that these results came from.
	Datacenter string

	// Failovers is a count of how many times we had to query a remote
	// datacenter.
	Failovers int
}

// PreparedQuery can be used to query the prepared query endpoints.
type PreparedQuery struct {
	c *Client
}

// PreparedQuery returns a handle to the prepared query endpoints.
func (c *Client) PreparedQuery() *PreparedQuery {
	return &PreparedQuery{c}
}

// Create makes a new prepared query. The ID of the new query is returned.
func (c *PreparedQuery) Create(query *PreparedQueryDefinition, q *WriteOptions) (string, *WriteMeta, error) {
	r := c.c.newRequest("POST", "/v1/query")
	r.setWriteOptions(q)
	r.obj = query
	rtt, resp, err := c.c.doRequest(r)
	if err != nil {
		return "", nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return "", nil, err
	}

	wm := &WriteMeta{}
	wm.RequestTime = rtt

	var out struct{ ID string }
	if err := decodeBody(resp, &out); err != nil {
		return "", nil, err
	}
	return out.ID, wm, nil
}

// Update makes updates to an existing prepared query.
func (c *PreparedQuery) Update(query *PreparedQueryDefinition, q *WriteOptions) (*WriteMeta, error) {
	return c.c.write("/v1/query/"+query.ID, query, nil, q)
}

// List is used to fetch all the prepared queries (always requires a management
// token).
func (c *PreparedQuery) List(q *QueryOptions) ([]*PreparedQueryDefinition, *QueryMeta, error) {
	var out []*PreparedQueryDefinition
	qm, err := c.c.query("/v1/query", &out, q)
	if err != nil {
		return nil, nil, err
	}
	return out, qm, nil
}

// Get is used to fetch a specific prepared query.
func (c *PreparedQuery) Get(queryID string, q *QueryOptions) ([]*PreparedQueryDefinition, *QueryMeta, error) {
	var out []*PreparedQueryDefinition
	qm, err := c.c.query("/v1/query/"+queryID, &out, q)
	if err != nil {
		return nil, nil, err
	}
	return out, qm, nil
}

// Delete is used to delete a specific prepared query.
func (c *PreparedQuery) Delete(queryID string, q *WriteOptions) (*WriteMeta, error) {
	r := c.c.newRequest("DELETE", "/v1/query/"+queryID)
	r.setWriteOptions(q)
	rtt, resp, err := c.c.doRequest(r)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, err
	}

	wm := &WriteMeta{}
	wm.RequestTime = rtt
	return wm, nil
}

// Execute is used to execute a specific prepared query. You can execute using
// a query ID or name.
func (c *PreparedQuery) Execute(queryIDOrName string, q *QueryOptions) (*PreparedQueryExecuteResponse, *QueryMeta, error) {
	var out *PreparedQueryExecuteResponse
	qm, err := c.c.query("/v1/query/"+queryIDOrName+"/execute", &out, q)
	if err != nil {
		return nil, nil, err
	}
	return out, qm, nil
}
