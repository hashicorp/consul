package proto

// IsRead is always true for QueryOption
func (q QueryOptions) IsRead() bool {
	return true
}

// AllowStaleRead returns whether a stale read should be allowed
func (q QueryOptions) AllowStaleRead() bool {
	return q.AllowStale
}

// TokenSecret returns the token to be used to authorize the request
func (q QueryOptions) TokenSecret() string {
	return q.Token
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
