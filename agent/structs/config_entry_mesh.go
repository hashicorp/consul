// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"encoding/json"
	"fmt"
	"strings"

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

	// ValidateClusters controls whether the clusters the route table refers to are validated. The default value is
	// false. When set to false and a route refers to a cluster that does not exist, the route table loads and routing
	// to a non-existent cluster results in a 404. When set to true and the route is set to a cluster that do not exist,
	// the route table will not load. For more information, refer to
	// [HTTP route configuration in the Envoy docs](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route.proto#envoy-v3-api-field-config-route-v3-routeconfiguration-validate-clusters)
	// for more details.
	ValidateClusters bool `json:",omitempty" alias:"validate_clusters"`

	TLS *MeshTLSConfig `json:",omitempty"`

	HTTP *MeshHTTPConfig `json:",omitempty"`

	Peering *PeeringMeshConfig `json:",omitempty"`

	Meta               map[string]string `json:",omitempty"`
	Hash               uint64            `json:",omitempty" hash:"ignore"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `hash:"ignore"`
}

func (e *MeshConfigEntry) SetHash(h uint64) {
	e.Hash = h
}

func (e *MeshConfigEntry) GetHash() uint64 {
	return e.Hash
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
	// Incoming configures settings for incoming HTTP traffic to mesh proxies.
	Incoming *MeshDirectionalHTTPConfig `json:",omitempty"`
	// There is not currently an outgoing MeshDirectionalHTTPConfig, as
	// the only required config for either direction at present is inbound
	// request normalization.
}

// MeshDirectionalHTTPConfig holds mesh configuration specific to HTTP
// requests for a given traffic direction.
type MeshDirectionalHTTPConfig struct {
	RequestNormalization *RequestNormalizationMeshConfig `json:",omitempty" alias:"request_normalization"`
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

// RequestNormalizationMeshConfig contains options pertaining to the
// normalization of HTTP requests processed by mesh proxies.
type RequestNormalizationMeshConfig struct {
	// InsecureDisablePathNormalization sets the value of the \`normalize_path\` option in the Envoy listener's
	// `HttpConnectionManager`. The default value is \`false\`. When set to \`true\` in Consul, \`normalize_path\` is
	// set to \`false\` for the Envoy proxy. This parameter disables the normalization of request URL paths according to
	// RFC 3986, conversion of \`\\\` to \`/\`, and decoding non-reserved %-encoded characters. When using L7 intentions
	// with path match rules, we recommend enabling path normalization in order to avoid match rule circumvention with
	// non-normalized path values.
	InsecureDisablePathNormalization bool `alias:"insecure_disable_path_normalization"`
	// MergeSlashes sets the value of the \`merge_slashes\` option in the Envoy listener's \`HttpConnectionManager\`.
	// The default value is \`false\`. This option controls the normalization of request URL paths by merging
	// consecutive \`/\` characters. This normalization is not part of RFC 3986. When using L7 intentions with path
	// match rules, we recommend enabling this setting to avoid match rule circumvention through non-normalized path
	// values, unless legitimate service traffic depends on allowing for repeat \`/\` characters, or upstream services
	// are configured to differentiate between single and multiple slashes.
	MergeSlashes bool `alias:"merge_slashes"`
	// PathWithEscapedSlashesAction sets the value of the \`path_with_escaped_slashes_action\` option in the Envoy
	// listener's \`HttpConnectionManager\`. The default value of this option is empty, which is equivalent to
	// \`IMPLEMENTATION_SPECIFIC_DEFAULT\`. This parameter controls the action taken in response to request URL paths
	// with escaped slashes in the path. When using L7 intentions with path match rules, we recommend enabling this
	// setting to avoid match rule circumvention through non-normalized path values, unless legitimate service traffic
	// depends on allowing for escaped \`/\` or \`\\\` characters, or upstream services are configured to differentiate
	// between escaped and unescaped slashes. Refer to the Envoy documentation for more information on available
	// options.
	PathWithEscapedSlashesAction PathWithEscapedSlashesAction `alias:"path_with_escaped_slashes_action"`
	// HeadersWithUnderscoresAction sets the value of the \`headers_with_underscores_action\` option in the Envoy
	// listener's \`HttpConnectionManager\` under \`common_http_protocol_options\`. The default value of this option is
	// empty, which is equivalent to \`ALLOW\`. Refer to the Envoy documentation for more information on available
	// options.
	HeadersWithUnderscoresAction HeadersWithUnderscoresAction `alias:"headers_with_underscores_action"`
}

// PathWithEscapedSlashesAction is an enum that defines the action to take when
// a request path contains escaped slashes. It mirrors exactly the set of options
// in Envoy's UriPathNormalizationOptions.PathWithEscapedSlashesAction enum.
type PathWithEscapedSlashesAction string

// See github.com/envoyproxy/go-control-plane envoy_http_v3.HttpConnectionManager_PathWithEscapedSlashesAction.
const (
	PathWithEscapedSlashesActionDefault             PathWithEscapedSlashesAction = "IMPLEMENTATION_SPECIFIC_DEFAULT"
	PathWithEscapedSlashesActionKeep                PathWithEscapedSlashesAction = "KEEP_UNCHANGED"
	PathWithEscapedSlashesActionReject              PathWithEscapedSlashesAction = "REJECT_REQUEST"
	PathWithEscapedSlashesActionUnescapeAndRedirect PathWithEscapedSlashesAction = "UNESCAPE_AND_REDIRECT"
	PathWithEscapedSlashesActionUnescapeAndForward  PathWithEscapedSlashesAction = "UNESCAPE_AND_FORWARD"
)

// PathWithEscapedSlashesActionStrings returns an ordered slice of all PathWithEscapedSlashesAction values as strings.
func PathWithEscapedSlashesActionStrings() []string {
	return []string{
		string(PathWithEscapedSlashesActionDefault),
		string(PathWithEscapedSlashesActionKeep),
		string(PathWithEscapedSlashesActionReject),
		string(PathWithEscapedSlashesActionUnescapeAndRedirect),
		string(PathWithEscapedSlashesActionUnescapeAndForward),
	}
}

// pathWithEscapedSlashesActions contains the canonical set of PathWithEscapedSlashesActionValues values.
var pathWithEscapedSlashesActions = (func() map[PathWithEscapedSlashesAction]struct{} {
	m := make(map[PathWithEscapedSlashesAction]struct{})
	for _, v := range PathWithEscapedSlashesActionStrings() {
		m[PathWithEscapedSlashesAction(v)] = struct{}{}
	}
	return m
})()

// HeadersWithUnderscoresAction is an enum that defines the action to take when
// a request contains headers with underscores. It mirrors exactly the set of
// options in Envoy's HttpProtocolOptions.HeadersWithUnderscoresAction enum.
type HeadersWithUnderscoresAction string

// See github.com/envoyproxy/go-control-plane envoy_core_v3.HttpProtocolOptions_HeadersWithUnderscoresAction.
const (
	HeadersWithUnderscoresActionAllow         HeadersWithUnderscoresAction = "ALLOW"
	HeadersWithUnderscoresActionRejectRequest HeadersWithUnderscoresAction = "REJECT_REQUEST"
	HeadersWithUnderscoresActionDropHeader    HeadersWithUnderscoresAction = "DROP_HEADER"
)

// HeadersWithUnderscoresActionStrings returns an ordered slice of all HeadersWithUnderscoresAction values as strings
// for use in returning validation errors.
func HeadersWithUnderscoresActionStrings() []string {
	return []string{
		string(HeadersWithUnderscoresActionAllow),
		string(HeadersWithUnderscoresActionRejectRequest),
		string(HeadersWithUnderscoresActionDropHeader),
	}
}

// headersWithUnderscoresActions contains the canonical set of HeadersWithUnderscoresAction values.
var headersWithUnderscoresActions = (func() map[HeadersWithUnderscoresAction]struct{} {
	m := make(map[HeadersWithUnderscoresAction]struct{})
	for _, v := range HeadersWithUnderscoresActionStrings() {
		m[HeadersWithUnderscoresAction(v)] = struct{}{}
	}
	return m
})()

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

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h

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

	if err := validateRequestNormalizationMeshConfig(e.GetHTTPIncomingRequestNormalization()); err != nil {
		return fmt.Errorf("error in HTTP incoming request normalization configuration: %v", err)
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

func (e *MeshConfigEntry) GetHTTP() *MeshHTTPConfig {
	if e == nil {
		return nil
	}
	return e.HTTP
}

func (e *MeshHTTPConfig) GetIncoming() *MeshDirectionalHTTPConfig {
	if e == nil {
		return nil
	}
	return e.Incoming
}

func (e *MeshDirectionalHTTPConfig) GetRequestNormalization() *RequestNormalizationMeshConfig {
	if e == nil {
		return nil
	}
	return e.RequestNormalization
}

// GetHTTPIncomingRequestNormalization is a convenience accessor for mesh.http.incoming.request_normalization
// since no other fields currently exist under mesh.http.incoming.
func (e *MeshConfigEntry) GetHTTPIncomingRequestNormalization() *RequestNormalizationMeshConfig {
	return e.GetHTTP().GetIncoming().GetRequestNormalization()
}

func (r *RequestNormalizationMeshConfig) GetInsecureDisablePathNormalization() bool {
	if r == nil {
		return false
	}
	return r.InsecureDisablePathNormalization
}

func (r *RequestNormalizationMeshConfig) GetMergeSlashes() bool {
	if r == nil {
		return false
	}
	return r.MergeSlashes
}

func (r *RequestNormalizationMeshConfig) GetPathWithEscapedSlashesAction() PathWithEscapedSlashesAction {
	if r == nil || r.PathWithEscapedSlashesAction == "" {
		return PathWithEscapedSlashesActionDefault
	}
	return r.PathWithEscapedSlashesAction
}

func (r *RequestNormalizationMeshConfig) GetHeadersWithUnderscoresAction() HeadersWithUnderscoresAction {
	if r == nil || r.HeadersWithUnderscoresAction == "" {
		return HeadersWithUnderscoresActionAllow
	}
	return r.HeadersWithUnderscoresAction
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

func validateRequestNormalizationMeshConfig(cfg *RequestNormalizationMeshConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validatePathWithEscapedSlashesAction(cfg.PathWithEscapedSlashesAction); err != nil {
		return err
	}
	if err := validateHeadersWithUnderscoresAction(cfg.HeadersWithUnderscoresAction); err != nil {
		return err
	}
	return nil
}

func validatePathWithEscapedSlashesAction(v PathWithEscapedSlashesAction) error {
	if v == "" {
		return nil
	}
	if _, ok := pathWithEscapedSlashesActions[v]; !ok {
		return fmt.Errorf("no matching PathWithEscapedSlashesAction value found for %s, please specify one of [%s]", string(v), strings.Join(PathWithEscapedSlashesActionStrings(), ", "))
	}
	return nil
}

func validateHeadersWithUnderscoresAction(v HeadersWithUnderscoresAction) error {
	if v == "" {
		return nil
	}
	if _, ok := headersWithUnderscoresActions[v]; !ok {
		return fmt.Errorf("no matching HeadersWithUnderscoresAction value found for %s, please specify one of [%s]", string(v), strings.Join(HeadersWithUnderscoresActionStrings(), ", "))
	}
	return nil
}
