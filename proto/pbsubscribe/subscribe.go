package pbsubscribe

import "time"

// RequestDatacenter implements structs.RPCInfo
func (req *SubscribeRequest) RequestDatacenter() string {
	return req.Datacenter
}

// IsRead implements structs.RPCInfo
func (req *SubscribeRequest) IsRead() bool {
	return true
}

// AllowStaleRead implements structs.RPCInfo
func (req *SubscribeRequest) AllowStaleRead() bool {
	return true
}

// TokenSecret implements structs.RPCInfo
func (req *SubscribeRequest) TokenSecret() string {
	return req.Token
}

// SetTokenSecret implements structs.RPCInfo
func (req *SubscribeRequest) SetTokenSecret(token string) {
	req.Token = token
}

// HasTimedOut implements structs.RPCInfo
func (req *SubscribeRequest) HasTimedOut(start time.Time, rpcHoldTimeout, maxQueryTime, defaultQueryTime time.Duration) (bool, error) {
	return time.Since(start) > rpcHoldTimeout, nil
}
