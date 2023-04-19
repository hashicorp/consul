// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/types"
)

type MeshConfigEntry struct {
	// TransparentProxy contains cluster-wide options pertaining to TPROXY mode
	// when enabled.
	TransparentProxy TransparentProxyMeshConfig `alias:"transparent_proxy"`

	// AllowEnablingPermissiveMutualTLS must be true in order to allow setting
	// MutualTLSMode=permissive in either service-defaults or proxy-defaults.
	AllowEnablingPermissiveMutualTLS bool `json:",omitempty" alias:"allow_enabling_permissive_mutual_tls"`

	TLS *MeshTLSConfig `json:",omitempty"`

	HTTP *MeshHTTPConfig `json:",omitempty"`

	Peering *PeeringMeshConfig `json:",omitempty"`

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

// TransparentProxyMeshConfig contains cluster-wide options pertaining to
// TPROXY mode when enabled.
type TransparentProxyMeshConfig struct {
	// MeshDestinationsOnly can be used to disable the pass-through that
	// allows traffic to destinations outside of the mesh.
	MeshDestinationsOnly bool `alias:"mesh_destinations_only"`
}

type MeshTLSConfig struct {
	Incoming *MeshDirectionalTLSConfig `json:",omitempty"`
	Outgoing *MeshDirectionalTLSConfig `json:",omitempty"`
}

type MeshDirectionalTLSConfig struct {
	TLSMinVersion types.TLSVersion `json:",omitempty" alias:"tls_min_version"`
	TLSMaxVersion types.TLSVersion `json:",omitempty" alias:"tls_max_version"`

	// Define a subset of cipher suites to restrict
	// Only applicable to connections negotiated via TLS 1.2 or earlier
	CipherSuites []types.TLSCipherSuite `json:",omitempty" alias:"cipher_suites"`
}

type MeshHTTPConfig struct {
	SanitizeXForwardedClientCert bool `alias:"sanitize_x_forwarded_client_cert"`
}

// PeeringMeshConfig contains cluster-wide options pertaining to peering.
type PeeringMeshConfig struct {
	// PeerThroughMeshGateways determines whether peering traffic between
	// control planes should flow through mesh gateways. If enabled,
	// Consul servers will advertise mesh gateway addresses as their own.
	// Additionally, mesh gateways will configure themselves to expose
	// the local servers using a peering-specific SNI.
	PeerThroughMeshGateways bool `alias:"peer_through_mesh_gateways"`
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

	if e.TLS != nil {
		if e.TLS.Incoming != nil {
			if err := validateMeshDirectionalTLSConfig(e.TLS.Incoming); err != nil {
				return fmt.Errorf("error in incoming TLS configuration: %v", err)
			}
		}
		if e.TLS.Outgoing != nil {
			if err := validateMeshDirectionalTLSConfig(e.TLS.Outgoing); err != nil {
				return fmt.Errorf("error in outgoing TLS configuration: %v", err)
			}
		}
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

func (e *MeshConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
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

func (e *MeshConfigEntry) PeerThroughMeshGateways() bool {
	if e == nil || e.Peering == nil {
		return false
	}
	return e.Peering.PeerThroughMeshGateways
}

func validateMeshDirectionalTLSConfig(cfg *MeshDirectionalTLSConfig) error {
	if cfg == nil {
		return nil
	}
	return validateTLSConfig(cfg.TLSMinVersion, cfg.TLSMaxVersion, cfg.CipherSuites)
}

func validateTLSConfig(
	tlsMinVersion types.TLSVersion,
	tlsMaxVersion types.TLSVersion,
	cipherSuites []types.TLSCipherSuite,
) error {
	if tlsMinVersion != types.TLSVersionUnspecified {
		if err := types.ValidateTLSVersion(tlsMinVersion); err != nil {
			return err
		}
	}

	if tlsMaxVersion != types.TLSVersionUnspecified {
		if err := types.ValidateTLSVersion(tlsMaxVersion); err != nil {
			return err
		}

		if tlsMinVersion != types.TLSVersionUnspecified {
			if err, maxLessThanMin := tlsMaxVersion.LessThan(tlsMinVersion); err == nil && maxLessThanMin {
				return fmt.Errorf("configuring max version %s less than the configured min version %s is invalid", tlsMaxVersion, tlsMinVersion)
			}
		}
	}

	if len(cipherSuites) != 0 {
		if _, ok := types.TLSVersionsWithConfigurableCipherSuites[tlsMinVersion]; !ok {
			return fmt.Errorf("configuring CipherSuites is only applicable to connections negotiated with TLS 1.2 or earlier, TLSMinVersion is set to %s", tlsMinVersion)
		}

		// NOTE: it would be nice to emit a warning but not return an error from
		// here if TLSMaxVersion is unspecified, TLS_AUTO or TLSv1_3
		if err := types.ValidateEnvoyCipherSuites(cipherSuites); err != nil {
			return err
		}
	}

	return nil
}
