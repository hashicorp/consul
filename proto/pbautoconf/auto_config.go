package pbautoconf

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
