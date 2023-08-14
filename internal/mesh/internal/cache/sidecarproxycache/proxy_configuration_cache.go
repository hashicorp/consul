package sidecarproxycache

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ProxyConfigurationCache tracks mappings between proxy configurations and proxy IDs
// that a configuration applies to. It is the responsibility of the controller to
// keep this cache up-to-date.
type ProxyConfigurationCache struct {
	mapper *bimapper.Mapper
}

func NewProxyConfigurationCache() *ProxyConfigurationCache {
	return &ProxyConfigurationCache{
		mapper: bimapper.New(types.ProxyConfigurationType, types.ProxyStateTemplateType),
	}
}

// ProxyConfigurationsByProxyID returns proxy configuration IDs given the id of the proxy state template.
func (c *ProxyConfigurationCache) ProxyConfigurationsByProxyID(id *pbresource.ID) []*pbresource.ID {
	return c.mapper.ItemIDsForLink(id)
}

// TrackProxyConfiguration tracks given proxy configuration ID and the linked proxy state template IDs.
func (c *ProxyConfigurationCache) TrackProxyConfiguration(proxyCfgID *pbresource.ID, proxyIDs []resource.ReferenceOrID) {
	c.mapper.TrackItem(proxyCfgID, proxyIDs)
}

// UntrackProxyConfiguration removes tracking for the given proxy configuration ID.
func (c *ProxyConfigurationCache) UntrackProxyConfiguration(proxyCfgID *pbresource.ID) {
	c.mapper.UntrackItem(proxyCfgID)
}

// UntrackProxyID removes tracking for the given proxy state template ID.
func (c *ProxyConfigurationCache) UntrackProxyID(proxyID *pbresource.ID) {
	c.mapper.UntrackLink(proxyID)
}
