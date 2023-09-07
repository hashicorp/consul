package meshv1alpha1

func (p *ProxyConfiguration) IsTransparentProxy() bool {
	if p.GetDynamicConfig() != nil &&
		p.DynamicConfig.Mode == ProxyMode_PROXY_MODE_TRANSPARENT {
		return true
	}

	return false
}
