package api

type ClusterConfigEntry struct {
	Kind             string
	Name             string
	Namespace        string                        `json:",omitempty"`
	TransparentProxy TransparentProxyClusterConfig `alias:"transparent_proxy"`
	Meta             map[string]string             `json:",omitempty"`
	CreateIndex      uint64
	ModifyIndex      uint64
}

type TransparentProxyClusterConfig struct {
	CatalogDestinationsOnly bool `alias:"catalog_destinations_only"`
}

func (e *ClusterConfigEntry) GetKind() string {
	return e.Kind
}

func (e *ClusterConfigEntry) GetName() string {
	return e.Name
}

func (e *ClusterConfigEntry) GetNamespace() string {
	return e.Namespace
}

func (e *ClusterConfigEntry) GetMeta() map[string]string {
	return e.Meta
}

func (e *ClusterConfigEntry) GetCreateIndex() uint64 {
	return e.CreateIndex
}

func (e *ClusterConfigEntry) GetModifyIndex() uint64 {
	return e.ModifyIndex
}
