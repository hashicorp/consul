package propertyoverride

import (
	"fmt"
	"strings"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/consul/lib/maps"
)

type stringSet map[string]struct{}

type propertyOverride struct {
	extensioncommon.BasicExtensionAdapter

	// Patches are an array of Patch operations to be applied to the target resource(s).
	Patches []Patch
	// Debug controls error messages when Path matching fails.
	// When set to true, all possible fields for the unmatched segment of the Path are returned.
	// When set to false, only the first ten possible fields are returned.
	Debug bool
	// ProxyType identifies the type of Envoy proxy that this extension applies to.
	// The extension will only be configured for proxies that match this type and
	// will be ignored for all other proxy types.
	ProxyType api.ServiceKind
}

// ResourceFilter matches specific Envoy resources to target with a Patch operation.
type ResourceFilter struct {
	// ResourceType specifies the Envoy resource type the patch applies to. Valid values are
	// `cluster`, `route`, `endpoint`, and `listener`.
	// This field is required.
	ResourceType ResourceType
	// TrafficDirection determines whether the patch will be applied to a service's inbound
	// or outbound resources.
	// This field is required.
	TrafficDirection TrafficDirection
}

// ResourceType is the type of Envoy resource being patched.
type ResourceType string

const (
	ResourceTypeCluster  ResourceType = "cluster"
	ResourceTypeListener ResourceType = "listener"
	ResourceTypeRoute    ResourceType = "route"
)

var ResourceTypes = stringSet{
	string(ResourceTypeCluster):  {},
	string(ResourceTypeListener): {},
	string(ResourceTypeRoute):    {},
}

// TrafficDirection determines whether inbound or outbound Envoy resources will be patched.
type TrafficDirection string

const (
	TrafficDirectionInbound  TrafficDirection = "inbound"
	TrafficDirectionOutbound TrafficDirection = "outbound"
)

var TrafficDirections = stringSet{string(TrafficDirectionInbound): {}, string(TrafficDirectionOutbound): {}}

// Op is the type of JSON Patch operation being applied.
type Op string

const (
	OpAdd    Op = "add"
	OpRemove Op = "remove"
)

var Ops = stringSet{string(OpAdd): {}, string(OpRemove): {}}

// validProxyTypes is the set of supported proxy types for this extension.
var validProxyTypes = stringSet{
	string(api.ServiceKindConnectProxy):       struct{}{},
	string(api.ServiceKindTerminatingGateway): struct{}{},
}

// Patch describes a single patch operation to modify the specific field of matching
// Envoy resources.
//
// The semantics of Patch closely resemble those of JSON Patch (https://jsonpatch.com/,
// https://datatracker.ietf.org/doc/html/rfc6902/).
type Patch struct {
	// ResourceFilter determines which Envoy resource(s) will be patched. ResourceFilter and
	// its subfields are not part of the JSON Patch specification.
	// This field is required.
	ResourceFilter ResourceFilter
	// Op represents the JSON Patch operation to be applied by the patch. Supported ops are
	// `add` and `remove`:
	//   - add: Replaces a field with the current value at the specified Path. (Note that
	//     JSON Patch does not inherently support object “merges”, which must be implemented
	//     using one discrete add per changed field.)
	//   - remove: Sets the value at the given path to `nil`. As with `add`, if the target
	//     field does not exist in the corresponding schema, an error is returned; this
	//     conforms to JSON Patch semantics and is intended to avoid silent failure when a
	//     field removal is expected.
	// This field is required.
	Op Op
	// Path specifies where the patch will be applied on a target resource. Path does not
	// support array member lookups or appending (`-`).
	//
	// When an unset but schema-valid (i.e. specified in the corresponding Envoy resource
	// .proto) intermediate message field is encountered on the Path, that field will be
	// set to its non-`nil` empty (default) value and evaluation will continue. This means
	// that even if parents of a field for a given Path are unset, a single patch can set
	// deeply nested children of that parent. Subsequent patching of these initialized
	// parent field(s) may be necessary to satisfy validation or configuration requirements.
	// This field is required.
	Path string
	// Value specifies the value that will be set at the given Path in a target resource in
	// an `add` operation.
	//
	// Value must be a map with scalar values, a scalar value, or an array of scalar values.
	// (Note that this along with the Path constraints noted above imply that setting values
	// nested within non-scalar arrays is not supported.)
	//
	// In every case, the target field will be replaced entirely with the specified value;
	// this conforms to JSON Patch `add` semantics. If Value is a map, the non-`nil` empty
	// value for the target field will be placed at the specified Path, and then the fields
	// specified in the Value map will be explicitly set.
	// This field is required if the Op is compatible with a Value per JSON Patch semantics
	// (e.g. `add`), and must not be set otherwise.
	Value any
}

var _ extensioncommon.BasicExtension = (*propertyOverride)(nil)

func (c *stringSet) checkRequired(v, fieldName string) error {
	if _, ok := (*c)[v]; !ok {
		if v == "" {
			return fmt.Errorf("field %s is required", fieldName)
		}
		return fmt.Errorf("invalid %s '%q'; supported values: %s",
			fieldName, v, strings.Join(maps.SliceOfKeys(*c), ", "))
	}
	return nil
}

func (f *ResourceFilter) validate() error {
	if f == nil || *f == (ResourceFilter{}) {
		return fmt.Errorf("field ResourceFilter is required")
	}
	if err := ResourceTypes.checkRequired(string(f.ResourceType), "ResourceType"); err != nil {
		return err
	}
	if err := TrafficDirections.checkRequired(string(f.TrafficDirection), "TrafficDirection"); err != nil {
		return err
	}
	return nil
}

// validate validates the fields of an individual Patch.
func (p *Patch) validate(debug bool) error {
	if err := p.ResourceFilter.validate(); err != nil {
		return err
	}

	if err := Ops.checkRequired(string(p.Op), "Op"); err != nil {
		return err
	}

	if p.Value != nil && p.Op != OpAdd {
		return fmt.Errorf("field Value is not supported for %s operation", p.Op)
	}

	// Attempt to execute the patch by applying it to a dummy empty struct.
	var err error
	switch p.ResourceFilter.ResourceType {
	case ResourceTypeCluster:
		_, err = PatchStruct(&envoy_cluster_v3.Cluster{}, *p, debug)
	case ResourceTypeRoute:
		_, err = PatchStruct(&envoy_route_v3.RouteConfiguration{}, *p, debug)
	case ResourceTypeListener:
		_, err = PatchStruct(&envoy_listener_v3.Listener{}, *p, debug)
	default:
		return fmt.Errorf("path validation unimplemented for %q", p.ResourceFilter.ResourceType)
	}

	return err
}

// validate validates the fields of the property-override extension, including all of its
// configured Patches.
func (p *propertyOverride) validate() error {
	if len(p.Patches) == 0 {
		return fmt.Errorf("at least one patch is required")
	}

	var resultErr error
	for _, patch := range p.Patches {
		if err := patch.validate(p.Debug); err != nil {
			resultErr = multierror.Append(resultErr, err)
		}
	}

	if err := validProxyTypes.checkRequired(string(p.ProxyType), "ProxyType"); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	return resultErr
}

// Constructor follows a specific function signature required for the extension registration.
// It constructs a BasicEnvoyExtender with a patch Extension from the arguments provided by ext.
func Constructor(ext api.EnvoyExtension) (extensioncommon.EnvoyExtender, error) {
	var p propertyOverride

	if name := ext.Name; name != api.BuiltinPropertyOverrideExtension {
		return nil, fmt.Errorf("expected extension name %q but got %q", api.BuiltinPropertyOverrideExtension, name)
	}
	if err := mapstructure.WeakDecode(ext.Arguments, &p); err != nil {
		return nil, fmt.Errorf("error decoding extension arguments: %v", err)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}

	return &extensioncommon.BasicEnvoyExtender{
		Extension: &p,
	}, nil
}

// CanApply returns true if the ProxyType of the extension config matches the kind of the local proxy indicated by the
// RuntimeConfig.
func (p *propertyOverride) CanApply(config *extensioncommon.RuntimeConfig) bool {
	return config.Kind == p.ProxyType
}

// PatchRoute patches the provided Envoy Route with any applicable `route` ResourceType patches.
func (p *propertyOverride) PatchRoute(_ *extensioncommon.RuntimeConfig, r *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	d := TrafficDirectionOutbound
	if extensioncommon.IsRouteToLocalAppCluster(r) {
		d = TrafficDirectionInbound
	}
	return patchResourceType[*envoy_route_v3.RouteConfiguration](r, p, ResourceTypeRoute, d, &defaultStructPatcher[*envoy_route_v3.RouteConfiguration]{})
}

// PatchCluster patches the provided Envoy Cluster with any applicable `cluster` ResourceType patches.
func (p *propertyOverride) PatchCluster(_ *extensioncommon.RuntimeConfig, c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	d := TrafficDirectionOutbound
	if extensioncommon.IsLocalAppCluster(c) {
		d = TrafficDirectionInbound
	}
	return patchResourceType[*envoy_cluster_v3.Cluster](c, p, ResourceTypeCluster, d, &defaultStructPatcher[*envoy_cluster_v3.Cluster]{})
}

// PatchListener patches the provided Envoy Listener with any applicable `listener` ResourceType patches.
func (p *propertyOverride) PatchListener(_ *extensioncommon.RuntimeConfig, l *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	d := TrafficDirectionOutbound
	if extensioncommon.IsInboundPublicListener(l) {
		d = TrafficDirectionInbound
	}
	return patchResourceType[*envoy_listener_v3.Listener](l, p, ResourceTypeListener, d, &defaultStructPatcher[*envoy_listener_v3.Listener]{})
}

// PatchFilter does nothing as this extension does not target Filters directly.
func (p *propertyOverride) PatchFilter(_ *extensioncommon.RuntimeConfig, f *envoy_listener_v3.Filter, _ bool) (*envoy_listener_v3.Filter, bool, error) {
	return f, false, nil
}

// patchResourceType applies Patches matching the given ResourceType to the target K.
// This helper simplifies implementation of the above per-type patch methods defined by BasicExtension.
func patchResourceType[K proto.Message](k K, p *propertyOverride, t ResourceType, d TrafficDirection, patcher structPatcher[K]) (K, bool, error) {
	resultPatched := false
	var resultErr error

	for _, patch := range p.Patches {
		if patch.ResourceFilter.ResourceType != t {
			continue
		}
		if patch.ResourceFilter.TrafficDirection != d {
			continue
		}
		newK, err := patcher.applyPatch(k, patch, p.Debug)
		if err != nil {
			resultErr = multierror.Append(resultErr, err)
		} else {
			k = newK
			resultPatched = true
		}
	}

	return k, resultPatched && resultErr == nil, resultErr
}

// structPatcher allows us to mock applyPatch in tests.
type structPatcher[K proto.Message] interface {
	applyPatch(k K, patch Patch, debug bool) (result K, e error)
}

type defaultStructPatcher[K proto.Message] struct {
}

func (patcher *defaultStructPatcher[K]) applyPatch(k K, patch Patch, debug bool) (result K, e error) {
	return PatchStruct(k, patch, debug)
}
