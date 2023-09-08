package meshv1alpha1

func (p *ProxyConfiguration) IsTransparentProxy() bool {
	return p.GetDynamicConfig() != nil &&
		p.DynamicConfig.Mode == ProxyMode_PROXY_MODE_TRANSPARENT
}
