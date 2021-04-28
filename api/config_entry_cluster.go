package api

type MeshConfigEntry struct {
	Kind             string
	Name             string
	Namespace        string                     `json:",omitempty"`
	TransparentProxy TransparentProxyMeshConfig `alias:"transparent_proxy"`
	Meta             map[string]string          `json:",omitempty"`
	CreateIndex      uint64
	ModifyIndex      uint64
}

type TransparentProxyMeshConfig struct {
	CatalogDestinationsOnly bool `alias:"catalog_destinations_only"`
}

func (e *MeshConfigEntry) GetKind() string {
	return e.Kind
}

func (e *MeshConfigEntry) GetName() string {
	return e.Name
}

func (e *MeshConfigEntry) GetNamespace() string {
	return e.Namespace
}

func (e *MeshConfigEntry) GetMeta() map[string]string {
	return e.Meta
}

func (e *MeshConfigEntry) GetCreateIndex() uint64 {
	return e.CreateIndex
}

func (e *MeshConfigEntry) GetModifyIndex() uint64 {
	return e.ModifyIndex
}
