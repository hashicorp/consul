package pbcommon

import (
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

// IsRead is always true for QueryOption
func (q *QueryOptions) IsRead() bool {
	return true
}

// AllowStaleRead returns whether a stale read should be allowed
func (q *QueryOptions) AllowStaleRead() bool {
	return q.AllowStale
}

func (q *QueryOptions) TokenSecret() string {
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

func (q QueryOptions) HasTimedOut(start time.Time, rpcHoldTimeout, maxQueryTime, defaultQueryTime time.Duration) bool {
	o := structs.QueryOptions{
		MaxQueryTime:  q.MaxQueryTime,
		MinQueryIndex: q.MinQueryIndex,
	}
	return o.HasTimedOut(start, rpcHoldTimeout, maxQueryTime, defaultQueryTime)
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

func (q *QueryMeta) GetBackend() structs.QueryBackend {
	return structs.QueryBackend(0)
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

func (w WriteRequest) HasTimedOut(start time.Time, rpcHoldTimeout, _, _ time.Duration) bool {
	return time.Since(start) > rpcHoldTimeout
}

func (td TargetDatacenter) RequestDatacenter() string {
	return td.Datacenter
}
