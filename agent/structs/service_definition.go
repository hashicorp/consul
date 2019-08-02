package structs

import (
	"github.com/hashicorp/go-multierror"
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
