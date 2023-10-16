package meshv2beta1

func (p *ComputedProxyConfiguration) IsTransparentProxy() bool {
	return p.GetDynamicConfig() != nil &&
		p.DynamicConfig.Mode == ProxyMode_PROXY_MODE_TRANSPARENT
}

func (p *ProxyConfiguration) IsTransparentProxy() bool {
	return p.GetDynamicConfig() != nil &&
		p.DynamicConfig.Mode == ProxyMode_PROXY_MODE_TRANSPARENT
}
