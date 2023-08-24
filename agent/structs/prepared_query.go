// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"strconv"

	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/types"
)

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

// AsTargets either returns Targets as is or Datacenters converted into
// Targets.
func (f *QueryFailoverOptions) AsTargets() []QueryFailoverTarget {
	if dcs := f.Datacenters; len(dcs) > 0 {
		var targets []QueryFailoverTarget
		for _, dc := range dcs {
			targets = append(targets, QueryFailoverTarget{Datacenter: dc})
		}
		return targets
	}

	return f.Targets
}

// IsEmpty returns true if the QueryFailoverOptions are empty (not set), false otherwise
func (f *QueryFailoverOptions) IsEmpty() bool {
	if f == nil || (f.NearestN == 0 && len(f.Datacenters) == 0 && len(f.Targets) == 0) {
		return true
	}
	return false
}

type QueryFailoverTarget struct {
	// Peer specifies a peer to try during failover.
	Peer string

	// Datacenter specifies a datacenter to try during failover.
	Datacenter string

	acl.EnterpriseMeta
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

	// SamenessGroup specifies a sameness group to query. The first member of the Sameness Group will
	// be targeted first on PQ execution and subsequent members will be targeted during failover scenarios.
	// This field is mutually exclusive with Failover.
	SamenessGroup string

	// Failover controls what we do if there are no healthy nodes in the
	// local datacenter.
	Failover QueryFailoverOptions

	// If OnlyPassing is true then we will only include nodes with passing
	// health checks (critical AND warning checks will cause a node to be
	// discarded)
	OnlyPassing bool

	// IgnoreCheckIDs is an optional list of health check IDs to ignore when
	// considering which nodes are healthy. It is useful as an emergency measure
	// to temporarily override some health check that is producing false negatives
	// for example.
	IgnoreCheckIDs []types.CheckID

	// Near allows the query to always prefer the node nearest the given
	// node. If the node does not exist, results are returned in their
	// normal randomly-shuffled order. Supplying the magic "_agent" value
	// is supported to sort near the agent which initiated the request.
	Near string

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

	// If not empty, Peer represents the peer that the service
	// was imported from.
	Peer string

	// EnterpriseMeta is the embedded enterprise metadata
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

const (
	// QueryTemplateTypeNamePrefixMatch uses the Name field of the query as
	// a prefix to select the template.
	QueryTemplateTypeNamePrefixMatch = "name_prefix_match"
)

// QueryTemplateOptions controls settings if this query is a template.
type QueryTemplateOptions struct {
	// Type, if non-empty, means that this query is a template. This is
	// set to one of the QueryTemplateType* constants above.
	Type string

	// Regexp is an optional regular expression to use to parse the full
	// name, once the prefix match has selected a template. This can be
	// used to extract parts of the name and choose a service name, set
	// tags, etc.
	Regexp string

	// RemoveEmptyTags, if true, removes empty tags from matched tag list
	RemoveEmptyTags bool
}

// PreparedQuery defines a complete prepared query, and is the structure we
// maintain in the state store.
type PreparedQuery struct {
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

	// Template is used to configure this query as a template, which will
	// respond to queries based on the Name, and then will be rendered
	// before it is executed.
	Template QueryTemplateOptions

	// Service defines a service query (leaving things open for other types
	// later).
	Service ServiceQuery

	// DNS has options that control how the results of this query are
	// served over DNS.
	DNS QueryDNSOptions

	RaftIndex
}

// GetACLPrefix returns the prefix to look up the prepared_query ACL policy for
// this query, and whether the prefix applies to this query. You always need to
// check the ok value before using the prefix.
func (pq *PreparedQuery) GetACLPrefix() (string, bool) {
	if pq.Name != "" || pq.Template.Type != "" {
		return pq.Name, true
	}

	return "", false
}

type PreparedQueries []*PreparedQuery

type IndexedPreparedQueries struct {
	Queries PreparedQueries
	QueryMeta
}

type PreparedQueryOp string

const (
	PreparedQueryCreate PreparedQueryOp = "create"
	PreparedQueryUpdate PreparedQueryOp = "update"
	PreparedQueryDelete PreparedQueryOp = "delete"
)

// QueryRequest is used to create or change prepared queries.
type PreparedQueryRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// Op is the operation to apply.
	Op PreparedQueryOp

	// Query is the query itself.
	Query *PreparedQuery

	// WriteRequest holds the ACL token to go along with this request.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (q *PreparedQueryRequest) RequestDatacenter() string {
	return q.Datacenter
}

// PreparedQuerySpecificRequest is used to get information about a prepared
// query.
type PreparedQuerySpecificRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// QueryID is the ID of a query.
	QueryID string

	// QueryOptions (unfortunately named here) controls the consistency
	// settings for the query lookup itself, as well as the service lookups.
	QueryOptions
}

// RequestDatacenter returns the datacenter for a given request.
func (q *PreparedQuerySpecificRequest) RequestDatacenter() string {
	return q.Datacenter
}

// PreparedQueryExecuteRequest is used to execute a prepared query.
type PreparedQueryExecuteRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// QueryIDOrName is the ID of a query _or_ the name of one, either can
	// be provided.
	QueryIDOrName string

	// Limit will trim the resulting list down to the given limit.
	Limit int

	// Connect will force results to be Connect-enabled nodes for the
	// matching services. This is equivalent in semantics exactly to
	// setting "Connect" in the query template itself, but allows callers
	// to use any prepared query in a Connect setting.
	Connect bool

	// Source is used to sort the results relative to a given node using
	// network coordinates.
	Source QuerySource

	// Agent is used to carry around a reference to the agent which initiated
	// the execute request. Used to distance-sort relative to the local node.
	Agent QuerySource

	// QueryOptions (unfortunately named here) controls the consistency
	// settings for the query lookup itself, as well as the service lookups.
	QueryOptions
}

// RequestDatacenter returns the datacenter for a given request.
func (q *PreparedQueryExecuteRequest) RequestDatacenter() string {
	return q.Datacenter
}

// CacheInfo implements cache.Request allowing requests to be cached on agent.
func (q *PreparedQueryExecuteRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          q.Token,
		Datacenter:     q.Datacenter,
		MinIndex:       q.MinQueryIndex,
		Timeout:        q.MaxQueryTime,
		MaxAge:         q.MaxAge,
		MustRevalidate: q.MustRevalidate,
	}

	// To calculate the cache key we hash over all the fields that affect the
	// output other than Datacenter and Token which are dealt with in the cache
	// framework already. Note the order here is important for the outcome - if we
	// ever care about cache-invalidation on updates e.g. because we persist
	// cached results, we need to be careful we maintain the same order of fields
	// here. We could alternatively use `hash:set` struct tag on an anonymous
	// struct to make it more robust if it becomes significant.
	v, err := hashstructure.Hash([]interface{}{
		q.QueryIDOrName,
		q.Limit,
		q.Connect,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// PreparedQueryExecuteRemoteRequest is used when running a local query in a
// remote datacenter.
type PreparedQueryExecuteRemoteRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// Query is a copy of the query to execute.  We have to ship the entire
	// query over since it won't be present in the remote state store.
	Query PreparedQuery

	// Limit will trim the resulting list down to the given limit.
	Limit int

	// Connect is the same as ExecuteRequest.
	Connect bool

	// QueryOptions (unfortunately named here) controls the consistency
	// settings for the service lookups.
	QueryOptions
}

// RequestDatacenter returns the datacenter for a given request.
func (q *PreparedQueryExecuteRemoteRequest) RequestDatacenter() string {
	return q.Datacenter
}

// PreparedQueryExecuteResponse has the results of executing a query.
type PreparedQueryExecuteResponse struct {
	// Service is the service that was queried.
	Service string

	// EnterpriseMeta of the service that was queried.
	acl.EnterpriseMeta

	// Nodes has the nodes that were output by the query.
	Nodes CheckServiceNodes

	// DNS has the options for serving these results over DNS.
	DNS QueryDNSOptions

	// Datacenter is the datacenter that these results came from.
	Datacenter string

	// PeerName specifies the cluster peer that these results came from.
	PeerName string

	// Failovers is a count of how many times we had to query a remote
	// datacenter.
	Failovers int

	// QueryMeta has freshness information about the query.
	QueryMeta
}

// PreparedQueryExplainResponse has the results when explaining a query/
type PreparedQueryExplainResponse struct {
	// Query has the fully-rendered query.
	Query PreparedQuery

	// QueryMeta has freshness information about the query.
	QueryMeta
}
