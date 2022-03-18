package structs

import (
	"time"
)

// QueryOptionsCompat is the interface that both the structs.QueryOptions
// and the proto/pbcommongogo.QueryOptions structs need to implement so that they
// can be operated on interchangeably
type QueryOptionsCompat interface {
	GetToken() string
	SetToken(string)
	GetMinQueryIndex() uint64
	SetMinQueryIndex(uint64)
	GetMaxQueryTime() (time.Duration, error)
	SetMaxQueryTime(time.Duration)
	GetAllowStale() bool
	SetAllowStale(bool)
	GetRequireConsistent() bool
	SetRequireConsistent(bool)
	GetUseCache() bool
	SetUseCache(bool)
	GetMaxStaleDuration() (time.Duration, error)
	SetMaxStaleDuration(time.Duration)
	GetMaxAge() (time.Duration, error)
	SetMaxAge(time.Duration)
	GetMustRevalidate() bool
	SetMustRevalidate(bool)
	GetStaleIfError() (time.Duration, error)
	SetStaleIfError(time.Duration)
	GetFilter() string
	SetFilter(string)
}

// QueryMetaCompat is the interface that both the structs.QueryMeta
// and the proto/pbcommongogo.QueryMeta structs need to implement so that they
// can be operated on interchangeably
type QueryMetaCompat interface {
	GetLastContact() (time.Duration, error)
	SetLastContact(time.Duration)
	GetKnownLeader() bool
	SetKnownLeader(bool)
	GetIndex() uint64
	SetIndex(uint64)
	GetConsistencyLevel() string
	SetConsistencyLevel(string)
	GetBackend() QueryBackend
	GetResultsFilteredByACLs() bool
	SetResultsFilteredByACLs(bool)
}

// GetToken helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetToken() string {
	if m != nil {
		return m.Token
	}
	return ""
}

// GetMinQueryIndex helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetMinQueryIndex() uint64 {
	if m != nil {
		return m.MinQueryIndex
	}
	return 0
}

// GetMaxQueryTime helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetMaxQueryTime() (time.Duration, error) {
	if m != nil {
		return m.MaxQueryTime, nil
	}
	return 0, nil
}

// GetAllowStale helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetAllowStale() bool {
	if m != nil {
		return m.AllowStale
	}
	return false
}

// GetRequireConsistent helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetRequireConsistent() bool {
	if m != nil {
		return m.RequireConsistent
	}
	return false
}

// GetUseCache helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetUseCache() bool {
	if m != nil {
		return m.UseCache
	}
	return false
}

// GetMaxStaleDuration helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetMaxStaleDuration() (time.Duration, error) {
	if m != nil {
		return m.MaxStaleDuration, nil
	}
	return 0, nil
}

// GetMaxAge helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetMaxAge() (time.Duration, error) {
	if m != nil {
		return m.MaxAge, nil
	}
	return 0, nil
}

// GetMustRevalidate helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetMustRevalidate() bool {
	if m != nil {
		return m.MustRevalidate
	}
	return false
}

// GetStaleIfError helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetStaleIfError() (time.Duration, error) {
	if m != nil {
		return m.StaleIfError, nil
	}
	return 0, nil
}

// GetFilter helps implement the QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryOptions) GetFilter() string {
	if m != nil {
		return m.Filter
	}
	return ""
}

// SetToken is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetToken(token string) {
	q.Token = token
}

// SetMinQueryIndex is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetMinQueryIndex(minQueryIndex uint64) {
	q.MinQueryIndex = minQueryIndex
}

// SetMaxQueryTime is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetMaxQueryTime(maxQueryTime time.Duration) {
	q.MaxQueryTime = maxQueryTime
}

// SetAllowStale is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetAllowStale(allowStale bool) {
	q.AllowStale = allowStale
}

// SetRequireConsistent is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetRequireConsistent(requireConsistent bool) {
	q.RequireConsistent = requireConsistent
}

// SetUseCache is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetUseCache(useCache bool) {
	q.UseCache = useCache
}

// SetMaxStaleDuration is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetMaxStaleDuration(maxStaleDuration time.Duration) {
	q.MaxStaleDuration = maxStaleDuration
}

// SetMaxAge is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetMaxAge(maxAge time.Duration) {
	q.MaxAge = maxAge
}

// SetMustRevalidate is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetMustRevalidate(mustRevalidate bool) {
	q.MustRevalidate = mustRevalidate
}

// SetStaleIfError is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetStaleIfError(staleIfError time.Duration) {
	q.StaleIfError = staleIfError
}

// SetFilter is needed to implement the structs.QueryOptionsCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryOptions) SetFilter(filter string) {
	q.Filter = filter
}

//
func (m *QueryMeta) GetIndex() uint64 {
	if m != nil {
		return m.Index
	}
	return 0
}

// GetLastContact helps implement the QueryMetaCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryMeta) GetLastContact() (time.Duration, error) {
	if m != nil {
		return m.LastContact, nil
	}
	return 0, nil
}

// GetKnownLeader helps implement the QueryMetaCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryMeta) GetKnownLeader() bool {
	if m != nil {
		return m.KnownLeader
	}
	return false
}

// GetConsistencyLevel helps implement the QueryMetaCompat interface
// Copied from proto/pbcommongogo/common.pb.go
func (m *QueryMeta) GetConsistencyLevel() string {
	if m != nil {
		return m.ConsistencyLevel
	}
	return ""
}

// SetLastContact is needed to implement the structs.QueryMetaCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryMeta) SetLastContact(lastContact time.Duration) {
	q.LastContact = lastContact
}

// SetKnownLeader is needed to implement the structs.QueryMetaCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryMeta) SetKnownLeader(knownLeader bool) {
	q.KnownLeader = knownLeader
}

// SetIndex is needed to implement the structs.QueryMetaCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryMeta) SetIndex(index uint64) {
	q.Index = index
}

// SetConsistencyLevel is needed to implement the structs.QueryMetaCompat interface
// Copied from proto/pbcommongogo/common.go
func (q *QueryMeta) SetConsistencyLevel(consistencyLevel string) {
	q.ConsistencyLevel = consistencyLevel
}

func (q *QueryMeta) GetBackend() QueryBackend {
	return q.Backend
}

// GetResultsFilteredByACLs is needed to implement the structs.QueryMetaCompat
// interface.
func (q *QueryMeta) GetResultsFilteredByACLs() bool {
	return q.ResultsFilteredByACLs
}

// SetResultsFilteredByACLs is needed to implement the structs.QueryMetaCompat
// interface.
func (q *QueryMeta) SetResultsFilteredByACLs(v bool) {
	q.ResultsFilteredByACLs = v
}
