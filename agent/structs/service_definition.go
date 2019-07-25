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
	TaggedAddresses   map[string]ServiceAddress
	Meta              map[string]string
	Port              int
	Check             CheckType
	Checks            CheckTypes
	Weights           *Weights
	Token             string
	EnableTagOverride bool

	// Proxy is the configuration set for Kind = connect-proxy. It is mandatory in
	// that case and an error to be set for any other kind. This config is part of
	// a proxy service definition. ProxyConfig may be a more natural name here, but
	// it's confusing for the UX because one of the fields in ConnectProxyConfig is
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
	if len(s.TaggedAddresses) > 0 {
		taggedAddrs := make(map[string]ServiceAddress)
		for k, v := range s.TaggedAddresses {
			taggedAddrs[k] = v
		}

		ns.TaggedAddresses = taggedAddrs
	}
	return ns
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
