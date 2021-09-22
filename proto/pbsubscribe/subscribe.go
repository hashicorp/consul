package pbsubscribe

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
