package serverlessplugin

import (
	"github.com/hashicorp/consul/agent/xds/xdscommon"
)

func MutateIndexedResources(resources *xdscommon.IndexedResources, config xdscommon.PluginConfiguration) (*xdscommon.IndexedResources, error) {
	return resources, nil
}
