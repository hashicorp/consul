package structs

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/mapstructure"
	"github.com/mitchellh/reflectwalk"
)

// ServiceDefinition is used to JSON decode the Service definitions. For
// documentation on specific fields see NodeService which is better documented.
type ServiceDefinition struct {
	Kind              ServiceKind `json:",omitempty"`
	ID                string
	Name              string
	Tags              []string
	Address           string
	Meta              map[string]string
	Port              int
	Check             CheckType
	Checks            CheckTypes
	Weights           *Weights
	Token             string
	EnableTagOverride bool
	// DEPRECATED (ProxyDestination) - remove this when removing ProxyDestination
	// ProxyDestination is deprecated in favour of Proxy.DestinationServiceName
	ProxyDestination string `json:",omitempty"`

	// Proxy is the configuration set for Kind = connect-proxy. It is mandatory in
	// that case and an error to be set for any other kind. This config is part of
	// a proxy service definition and is distinct from but shares some fields with
	// the Connect.Proxy which configures a managed proxy as part of the actual
	// service's definition. This duplication is ugly but seemed better than the
	// alternative which was to re-use the same struct fields for both cases even
	// though the semantics are different and the non-shared fields make no sense
	// in the other case. ProxyConfig may be a more natural name here, but it's
	// confusing for the UX because one of the fields in ConnectProxyConfig is
	// also called just "Config"
	Proxy *ConnectProxyConfig

	Connect *ServiceConnect
}

func (s *ServiceDefinition) NodeService() *NodeService {
	ns := &NodeService{
		Kind:              s.Kind,
		ID:                s.ID,
		Service:           s.Name,
		Tags:              s.Tags,
		Address:           s.Address,
		Meta:              s.Meta,
		Port:              s.Port,
		Weights:           s.Weights,
		EnableTagOverride: s.EnableTagOverride,
	}
	if s.Connect != nil {
		ns.Connect = *s.Connect
	}
	if s.Proxy != nil {
		ns.Proxy = *s.Proxy
		// Ensure the Upstream type is defaulted
		for i := range ns.Proxy.Upstreams {
			if ns.Proxy.Upstreams[i].DestinationType == "" {
				ns.Proxy.Upstreams[i].DestinationType = UpstreamDestTypeService
			}
		}
	} else {
		// DEPRECATED (ProxyDestination) - remove this when removing ProxyDestination
		// Legacy convert ProxyDestination into a Proxy config
		ns.Proxy.DestinationServiceName = s.ProxyDestination
	}
	if ns.ID == "" && ns.Service != "" {
		ns.ID = ns.Service
	}
	return ns
}

// ConnectManagedProxy returns a ConnectManagedProxy from the ServiceDefinition
// if one is configured validly. Note that is may return nil if no proxy is
// configured and will also return nil error in this case too as it's an
// expected case. The error returned indicates that there was an attempt to
// configure a proxy made but that it was invalid input, e.g. invalid
// "exec_mode".
func (s *ServiceDefinition) ConnectManagedProxy() (*ConnectManagedProxy, error) {
	if s.Connect == nil || s.Connect.Proxy == nil {
		return nil, nil
	}

	// NodeService performs some simple normalization like copying ID from Name
	// which we shouldn't hard code ourselves here...
	ns := s.NodeService()

	execMode, err := NewProxyExecMode(s.Connect.Proxy.ExecMode)
	if err != nil {
		return nil, err
	}

	// If upstreams were set in the config and NOT in the actual Upstreams field,
	// extract them out to the new explicit Upstreams and unset in config to make
	// transition smooth.
	if deprecatedUpstreams, ok := s.Connect.Proxy.Config["upstreams"]; ok {
		if len(s.Connect.Proxy.Upstreams) == 0 {
			if slice, ok := deprecatedUpstreams.([]interface{}); ok {
				for _, raw := range slice {
					var oldU deprecatedBuiltInProxyUpstreamConfig
					var decMeta mapstructure.Metadata
					decCfg := &mapstructure.DecoderConfig{
						Metadata: &decMeta,
						Result:   &oldU,
					}
					dec, err := mapstructure.NewDecoder(decCfg)
					if err != nil {
						// Just skip it - we never used to parse this so never failed
						// invalid stuff till it hit the proxy. This is a best-effort
						// attempt to not break existing service definitions so it's not the
						// end of the world if we don't have exactly the same failure mode
						// for invalid input.
						continue
					}
					err = dec.Decode(raw)
					if err != nil {
						// same logic as above
						continue
					}

					newT := UpstreamDestTypeService
					if oldU.DestinationType == "prepared_query" {
						newT = UpstreamDestTypePreparedQuery
					}
					u := Upstream{
						DestinationType:      newT,
						DestinationName:      oldU.DestinationName,
						DestinationNamespace: oldU.DestinationNamespace,
						Datacenter:           oldU.DestinationDatacenter,
						LocalBindAddress:     oldU.LocalBindAddress,
						LocalBindPort:        oldU.LocalBindPort,
					}
					// Any unrecognized keys should be copied into the config map
					if len(decMeta.Unused) > 0 {
						u.Config = make(map[string]interface{})
						// Paranoid type assertion - mapstructure would have errored if this
						// wasn't safe but panics are bad...
						if rawMap, ok := raw.(map[string]interface{}); ok {
							for _, k := range decMeta.Unused {
								u.Config[k] = rawMap[k]
							}
						}
					}
					s.Connect.Proxy.Upstreams = append(s.Connect.Proxy.Upstreams, u)
				}
			}
		}
		// Remove upstreams even if we didn't add them for consistency.
		delete(s.Connect.Proxy.Config, "upstreams")
	}

	p := &ConnectManagedProxy{
		ExecMode:  execMode,
		Command:   s.Connect.Proxy.Command,
		Config:    s.Connect.Proxy.Config,
		Upstreams: s.Connect.Proxy.Upstreams,
		// ProxyService will be setup when the agent registers the configured
		// proxies and starts them etc.
		TargetServiceID: ns.ID,
	}

	// Ensure the Upstream type is defaulted
	for i := range p.Upstreams {
		if p.Upstreams[i].DestinationType == "" {
			p.Upstreams[i].DestinationType = UpstreamDestTypeService
		}
	}

	return p, nil
}

// deprecatedBuiltInProxyUpstreamConfig is a struct for extracting old
// connect/proxy.UpstreamConfiguration syntax upstreams from existing managed
// proxy configs to convert them to new first-class Upstreams.
type deprecatedBuiltInProxyUpstreamConfig struct {
	LocalBindAddress      string `json:"local_bind_address" hcl:"local_bind_address,attr" mapstructure:"local_bind_address"`
	LocalBindPort         int    `json:"local_bind_port" hcl:"local_bind_port,attr" mapstructure:"local_bind_port"`
	DestinationName       string `json:"destination_name" hcl:"destination_name,attr" mapstructure:"destination_name"`
	DestinationNamespace  string `json:"destination_namespace" hcl:"destination_namespace,attr" mapstructure:"destination_namespace"`
	DestinationType       string `json:"destination_type" hcl:"destination_type,attr" mapstructure:"destination_type"`
	DestinationDatacenter string `json:"destination_datacenter" hcl:"destination_datacenter,attr" mapstructure:"destination_datacenter"`
	// ConnectTimeoutMs is removed explicitly because any additional config we
	// find including this field should be put into the opaque Config map in
	// Upstream.
}

// Validate validates the service definition. This also calls the underlying
// Validate method on the NodeService.
//
// NOTE(mitchellh): This currently only validates fields related to Connect
// and is incomplete with regards to other fields.
func (s *ServiceDefinition) Validate() error {
	var result error

	if s.Kind == ServiceKindTypical {
		if s.Connect != nil {
			if s.Connect.Proxy != nil {
				if s.Connect.Native {
					result = multierror.Append(result, fmt.Errorf(
						"Services that are Connect native may not have a proxy configuration"))
				}

				if s.Port == 0 {
					result = multierror.Append(result, fmt.Errorf(
						"Services with a Connect managed proxy must have a port set"))
				}
			}
		}
	}

	// Validate the NodeService which covers a lot
	if err := s.NodeService().Validate(); err != nil {
		result = multierror.Append(result, err)
	}

	return result
}

func (s *ServiceDefinition) CheckTypes() (checks CheckTypes, err error) {
	if !s.Check.Empty() {
		err := s.Check.Validate()
		if err != nil {
			return nil, err
		}
		checks = append(checks, &s.Check)
	}
	for _, check := range s.Checks {
		if err := check.Validate(); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, nil
}

// ServiceDefinitionConnectProxy is the connect proxy config  within a service
// registration. Note this is duplicated in config.ServiceConnectProxy and needs
// to be kept in sync.
type ServiceDefinitionConnectProxy struct {
	Command   []string               `json:",omitempty"`
	ExecMode  string                 `json:",omitempty"`
	Config    map[string]interface{} `json:",omitempty"`
	Upstreams []Upstream             `json:",omitempty"`
}

func (p *ServiceDefinitionConnectProxy) MarshalJSON() ([]byte, error) {
	type typeCopy ServiceDefinitionConnectProxy
	copy := typeCopy(*p)

	// If we have config, then we want to run it through our proxyConfigWalker
	// which is a reflectwalk implementation that attempts to turn arbitrary
	// interface{} values into JSON-safe equivalents (more or less). This
	// should always work because the config input is either HCL or JSON and
	// both are JSON compatible.
	if copy.Config != nil {
		configCopyRaw, err := copystructure.Copy(copy.Config)
		if err != nil {
			return nil, err
		}
		configCopy, ok := configCopyRaw.(map[string]interface{})
		if !ok {
			// This should never fail because we KNOW the input type,
			// but we don't ever want to risk the panic.
			return nil, fmt.Errorf("internal error: config copy is not right type")
		}
		if err := reflectwalk.Walk(configCopy, &proxyConfigWalker{}); err != nil {
			return nil, err
		}

		copy.Config = configCopy
	}

	return json.Marshal(&copy)
}

var typMapIfaceIface = reflect.TypeOf(map[interface{}]interface{}{})

// proxyConfigWalker implements interfaces for the reflectwalk package
// (github.com/mitchellh/reflectwalk) that can be used to automatically
// make the proxy configuration safe for JSON usage.
//
// Most of the implementation here is just keeping track of where we are
// in the reflectwalk process, so that we can replace values. The key logic
// is in Slice() and SliceElem().
//
// In particular we're looking to replace two cases the msgpack codec causes:
//
//   1.) String values get turned into byte slices. JSON will base64-encode
//       this and we don't want that, so we convert them back to strings.
//
//   2.) Nested maps turn into map[interface{}]interface{}. JSON cannot
//       encode this, so we need to turn it back into map[string]interface{}.
//
// This is tested via the TestServiceDefinitionConnectProxy_json test.
type proxyConfigWalker struct {
	lastValue    reflect.Value        // lastValue of map, required for replacement
	loc, lastLoc reflectwalk.Location // locations
	cs           []reflect.Value      // container stack
	csKey        []reflect.Value      // container keys (maps) stack
	csData       interface{}          // current container data
	sliceIndex   []int                // slice index stack (one for each slice in cs)
}

func (w *proxyConfigWalker) Enter(loc reflectwalk.Location) error {
	w.lastLoc = w.loc
	w.loc = loc
	return nil
}

func (w *proxyConfigWalker) Exit(loc reflectwalk.Location) error {
	w.loc = reflectwalk.None
	w.lastLoc = reflectwalk.None

	switch loc {
	case reflectwalk.Map:
		w.cs = w.cs[:len(w.cs)-1]
	case reflectwalk.MapValue:
		w.csKey = w.csKey[:len(w.csKey)-1]
	case reflectwalk.Slice:
		// Split any values that need to be split
		w.cs = w.cs[:len(w.cs)-1]
	case reflectwalk.SliceElem:
		w.csKey = w.csKey[:len(w.csKey)-1]
		w.sliceIndex = w.sliceIndex[:len(w.sliceIndex)-1]
	}

	return nil
}

func (w *proxyConfigWalker) Map(m reflect.Value) error {
	w.cs = append(w.cs, m)
	return nil
}

func (w *proxyConfigWalker) MapElem(m, k, v reflect.Value) error {
	w.csData = k
	w.csKey = append(w.csKey, k)

	w.lastValue = v
	return nil
}

func (w *proxyConfigWalker) Slice(v reflect.Value) error {
	// If we find a []byte slice, it is an HCL-string converted to []byte.
	// Convert it back to a Go string and replace the value so that JSON
	// doesn't base64-encode it.
	if v.Type() == reflect.TypeOf([]byte{}) {
		resultVal := reflect.ValueOf(string(v.Interface().([]byte)))
		switch w.lastLoc {
		case reflectwalk.MapKey:
			m := w.cs[len(w.cs)-1]

			// Delete the old value
			var zero reflect.Value
			m.SetMapIndex(w.csData.(reflect.Value), zero)

			// Set the new key with the existing value
			m.SetMapIndex(resultVal, w.lastValue)

			// Set the key to be the new key
			w.csData = resultVal
		case reflectwalk.MapValue:
			// If we're in a map, then the only way to set a map value is
			// to set it directly.
			m := w.cs[len(w.cs)-1]
			mk := w.csData.(reflect.Value)
			m.SetMapIndex(mk, resultVal)
		case reflectwalk.Slice:
			s := w.cs[len(w.cs)-1]
			s.Index(w.sliceIndex[len(w.sliceIndex)-1]).Set(resultVal)
		default:
			return fmt.Errorf("cannot convert []byte")
		}
	}

	w.cs = append(w.cs, v)
	return nil
}

func (w *proxyConfigWalker) SliceElem(i int, elem reflect.Value) error {
	w.csKey = append(w.csKey, reflect.ValueOf(i))
	w.sliceIndex = append(w.sliceIndex, i)

	// We're looking specifically for map[interface{}]interface{}, but the
	// values in a slice are wrapped up in interface{} so we need to unwrap
	// that first. Therefore, we do three checks: 1.) is it valid? so we
	// don't panic, 2.) is it an interface{}? so we can unwrap it and 3.)
	// after unwrapping the interface do we have the map we expect?
	if !elem.IsValid() {
		return nil
	}

	if elem.Kind() != reflect.Interface {
		return nil
	}

	if inner := elem.Elem(); inner.Type() == typMapIfaceIface {
		// map[interface{}]interface{}, attempt to weakly decode into string keys
		var target map[string]interface{}
		if err := mapstructure.WeakDecode(inner.Interface(), &target); err != nil {
			return err
		}

		elem.Set(reflect.ValueOf(target))
	}

	return nil
}
