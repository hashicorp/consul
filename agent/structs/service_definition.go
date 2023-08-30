package structs

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/lib"
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
	SocketPath        string
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

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`

	Connect *ServiceConnect
}

func (t *ServiceDefinition) UnmarshalJSON(data []byte) (err error) {
	type Alias ServiceDefinition

	aux := &struct {
		EnableTagOverrideSnake bool                      `json:"enable_tag_override"`
		TaggedAddressesSnake   map[string]ServiceAddress `json:"tagged_addresses"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if aux.EnableTagOverrideSnake {
		t.EnableTagOverride = aux.EnableTagOverrideSnake
	}
	if len(t.TaggedAddresses) == 0 {
		t.TaggedAddresses = aux.TaggedAddressesSnake
	}

	return nil
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
		SocketPath:        s.SocketPath,
		Weights:           s.Weights,
		EnableTagOverride: s.EnableTagOverride,
		EnterpriseMeta:    s.EnterpriseMeta,
	}
	ns.EnterpriseMeta.Normalize()

	if s.Connect != nil {
		ns.Connect = *s.Connect
	}
	if s.Proxy != nil {
		ns.Proxy = *s.Proxy
		for i := range ns.Proxy.Upstreams {
			// Ensure the Upstream type is defaulted
			if ns.Proxy.Upstreams[i].DestinationType == "" {
				ns.Proxy.Upstreams[i].DestinationType = UpstreamDestTypeService
			}

			// If a proxy's namespace and partition are not defined, inherit from the proxied service
			// Applicable only to Consul Enterprise.
			if ns.Proxy.Upstreams[i].DestinationNamespace == "" {
				ns.Proxy.Upstreams[i].DestinationNamespace = ns.EnterpriseMeta.NamespaceOrEmpty()
			}
			if ns.Proxy.Upstreams[i].DestinationPartition == "" {
				ns.Proxy.Upstreams[i].DestinationPartition = ns.EnterpriseMeta.PartitionOrEmpty()
			}
		}
		ns.Proxy.Expose = s.Proxy.Expose
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

	// Validate the NodeService which covers a lot
	if err := s.NodeService().Validate(); err != nil {
		result = multierror.Append(result, err)
	}
	for _, c := range s.Checks {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("check %q: %s", c.Name, err)
		}
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
