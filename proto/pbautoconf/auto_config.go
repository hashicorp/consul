package pbautoconf

import "time"

func (req *AutoConfigRequest) RequestDatacenter() string {
	return req.Datacenter
}

func (req *AutoConfigRequest) IsRead() bool {
	return false
}

func (req *AutoConfigRequest) AllowStaleRead() bool {
	return false
}

func (req *AutoConfigRequest) TokenSecret() string {
	return req.ConsulToken
}

func (req *AutoConfigRequest) SetTokenSecret(token string) {
	req.ConsulToken = token
}

func (req *AutoConfigRequest) HasTimedOut(start time.Time, rpcHoldTimeout, maxQueryTime, defaultQueryTime time.Duration) bool {
	return time.Since(start) > rpcHoldTimeout
}
