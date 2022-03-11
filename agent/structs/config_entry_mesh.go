package structs

import (
	"encoding/json"
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
	// MeshDestinationsOnly can be used to disable the pass-through that
	// allows traffic to destinations outside of the mesh.
	MeshDestinationsOnly bool `alias:"mesh_destinations_only"`
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

func (e *MeshConfigEntry) CanRead(authz acl.Authorizer) error {
	return nil
}

func (e *MeshConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
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

// MarshalJSON adds the Kind field so that the JSON can be decoded back into the
// correct type.
// This method is implemented on the structs type (as apposed to the api type)
// because that is what the API currently uses to return a response.
func (e *MeshConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias MeshConfigEntry
	source := &struct {
		Kind string
		*Alias
	}{
		Kind:  MeshConfig,
		Alias: (*Alias)(e),
	}
	return json.Marshal(source)
}
