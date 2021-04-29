package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

type MeshConfigEntry struct {
	// TransparentProxy contains cluster-wide options pertaining to TPROXY mode
	// when enabled.
	TransparentProxy TransparentProxyMeshConfig `alias:"transparent_proxy"`

	Meta           map[string]string `json:",omitempty"`
	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

// TransparentProxyMeshConfig contains cluster-wide options pertaining to
// TPROXY mode when enabled.
type TransparentProxyMeshConfig struct {
	// CatalogDestinationsOnly can be used to disable the pass-through that
	// allows traffic to destinations outside of the mesh.
	CatalogDestinationsOnly bool `alias:"catalog_destinations_only"`
}

func (e *MeshConfigEntry) GetKind() string {
	return MeshConfig
}

func (e *MeshConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return MeshConfigMesh
}

func (e *MeshConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *MeshConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.EnterpriseMeta.Normalize()
	return nil
}

func (e *MeshConfigEntry) Validate() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}
	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	return e.validateEnterpriseMeta()
}

func (e *MeshConfigEntry) CanRead(authz acl.Authorizer) bool {
	return true
}

func (e *MeshConfigEntry) CanWrite(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.OperatorWrite(&authzContext) == acl.Allow
}

func (e *MeshConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *MeshConfigEntry) GetEnterpriseMeta() *EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}
