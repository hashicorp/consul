package pbcommon

import (
	"strconv"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/mitchellh/hashstructure"
)

// IsRead is always true for QueryOption
func (q *QueryOptions) IsRead() bool {
	return true
}

// AllowStaleRead returns whether a stale read should be allowed
func (q *QueryOptions) AllowStaleRead() bool {
	if q == nil {
		return false
	}

	return q.AllowStale
}

func (q *QueryOptions) TokenSecret() string {
	if q == nil {
		return ""
	}

	return q.Token
}

func (q *QueryOptions) SetTokenSecret(s string) {
	q.Token = s
}

// SetToken is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetToken(token string) {
	q.Token = token
}

// SetMinQueryIndex is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetMinQueryIndex(minQueryIndex uint64) {
	q.MinQueryIndex = minQueryIndex
}

// SetMaxQueryTime is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetMaxQueryTime(maxQueryTime time.Duration) {
	q.MaxQueryTime = maxQueryTime
}

// SetAllowStale is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetAllowStale(allowStale bool) {
	q.AllowStale = allowStale
}

// SetRequireConsistent is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetRequireConsistent(requireConsistent bool) {
	q.RequireConsistent = requireConsistent
}

// SetUseCache is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetUseCache(useCache bool) {
	q.UseCache = useCache
}

// SetMaxStaleDuration is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetMaxStaleDuration(maxStaleDuration time.Duration) {
	q.MaxStaleDuration = maxStaleDuration
}

// SetMaxAge is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetMaxAge(maxAge time.Duration) {
	q.MaxAge = maxAge
}

// SetMustRevalidate is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetMustRevalidate(mustRevalidate bool) {
	q.MustRevalidate = mustRevalidate
}

// SetStaleIfError is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetStaleIfError(staleIfError time.Duration) {
	q.StaleIfError = staleIfError
}

// SetFilter is needed to implement the structs.QueryOptionsCompat interface
func (q *QueryOptions) SetFilter(filter string) {
	q.Filter = filter
}

// SetLastContact is needed to implement the structs.QueryMetaCompat interface
func (q *QueryMeta) SetLastContact(lastContact time.Duration) {
	q.LastContact = lastContact
}

// SetKnownLeader is needed to implement the structs.QueryMetaCompat interface
func (q *QueryMeta) SetKnownLeader(knownLeader bool) {
	q.KnownLeader = knownLeader
}

// SetIndex is needed to implement the structs.QueryMetaCompat interface
func (q *QueryMeta) SetIndex(index uint64) {
	q.Index = index
}

// SetConsistencyLevel is needed to implement the structs.QueryMetaCompat interface
func (q *QueryMeta) SetConsistencyLevel(consistencyLevel string) {
	q.ConsistencyLevel = consistencyLevel
}

// WriteRequest only applies to writes, always false
func (w WriteRequest) IsRead() bool {
	return false
}

func (w WriteRequest) TokenSecret() string {
	return w.Token
}

func (w *WriteRequest) SetTokenSecret(s string) {
	w.Token = s
}

// AllowStaleRead returns whether a stale read should be allowed
func (w WriteRequest) AllowStaleRead() bool {
	return false
}

func (td TargetDatacenter) RequestDatacenter() string {
	return td.Datacenter
}

func (r *DCSpecificRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.QueryOptions.GetToken(),
		Datacenter:     r.Datacenter,
		MinIndex:       r.QueryOptions.GetMinQueryIndex(),
		Timeout:        r.QueryOptions.GetMaxQueryTime(),
		MaxAge:         r.QueryOptions.GetMaxAge(),
		MustRevalidate: r.QueryOptions.GetMustRevalidate(),
	}

	// To calculate the cache key we only hash the node meta filters and the bexpr filter.
	// The datacenter is handled by the cache framework. The other fields are
	// not, but should not be used in any cache types.
	v, err := hashstructure.Hash([]interface{}{
		r.NodeMetaFilters,
		r.QueryOptions.GetFilter(),
		r.EnterpriseMeta,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

func (r *DCSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}
