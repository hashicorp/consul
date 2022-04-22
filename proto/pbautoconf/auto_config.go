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

func (req *AutoConfigRequest) HasTimedOut(start time.Time, rpcHoldTimeout, maxQueryTime, defaultQueryTime time.Duration) (bool, error) {
	return time.Since(start) > req.Timeout(rpcHoldTimeout, maxQueryTime, defaultQueryTime), nil
}

func (req *AutoConfigRequest) Timeout(rpcHoldTimeout, maxQueryTime, defaultQueryTime time.Duration) time.Duration {
	return rpcHoldTimeout
}
