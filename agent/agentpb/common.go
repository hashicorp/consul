package agentpb

import (
	"time"
)

// IsRead is always true for QueryOption
func (q *QueryOptions) IsRead() bool {
	return true
}

// AllowStaleRead returns whether a stale read should be allowed
func (q *QueryOptions) AllowStaleRead() bool {
	return q.AllowStale
}

// TokenSecret returns the token to be used to authorize the request
func (q *QueryOptions) TokenSecret() string {
	return q.Token
}

// GetMinQueryIndex implements the interface necessary to be used
// in a blocking query
func (q *QueryOptions) GetMinQueryIndex() uint64 {
	return q.MinQueryIndex
}

// GetMaxQueryTime implements the interface necessary to be used
// in a blocking query
func (q *QueryOptions) GetMaxQueryTime() time.Duration {
	return q.MaxQueryTime
}

// GetRequireConsistent implements the interface necessary to be used
// in a blocking query
func (q *QueryOptions) GetRequireConsistent() bool {
	return q.RequireConsistent
}

// SetLastContact implements the interface necessary to be used
// in a blocking query
func (q *QueryMeta) SetLastContact(lastContact time.Duration) {
	q.LastContact = lastContact
}

// SetKnownLeader implements the interface necessary to be used
// in a blocking query
func (q *QueryMeta) SetKnownLeader(knownLeader bool) {
	q.KnownLeader = knownLeader
}

// GetIndex implements the interface necessary to be used
// in a blocking query
func (q *QueryMeta) GetIndex() uint64 {
	return q.Index
}

// SetIndex implements the interface necessary to be used
// in a blocking query
func (q *QueryMeta) SetIndex(index uint64) {
	q.Index = index
}

// WriteRequest only applies to writes, always false
func (w WriteRequest) IsRead() bool {
	return false
}

// AllowStaleRead returns whether a stale read should be allowed
func (w WriteRequest) AllowStaleRead() bool {
	return false
}

// TokenSecret returns the token to be used to authorize the request
func (w WriteRequest) TokenSecret() string {
	return w.Token
}

func (td TargetDatacenter) RequestDatacenter() string {
	return td.Datacenter
}
