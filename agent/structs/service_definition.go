package structs

// ServiceDefinition is used to JSON decode the Service definitions. For
// documentation on specific fields see NodeService which is better documented.
type ServiceDefinition struct {
	Kind              ServiceKind
	ID                string
	Name              string
	Tags              []string
	Address           string
	Meta              map[string]string
	Port              int
	Check             CheckType
	Checks            CheckTypes
	Token             string
	EnableTagOverride bool
	ProxyDestination  string
	Connect           *ServiceDefinitionConnect
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
		EnableTagOverride: s.EnableTagOverride,
		ProxyDestination:  s.ProxyDestination,
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

	p := &ConnectManagedProxy{
		ExecMode: execMode,
		Command:  s.Connect.Proxy.Command,
		Config:   s.Connect.Proxy.Config,
		// ProxyService will be setup when the agent registers the configured
		// proxies and starts them etc.
		TargetServiceID: ns.ID,
	}

	return p, nil
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

// ServiceDefinitionConnect is the connect block within a service registration.
// Note this is duplicated in config.ServiceConnect and needs to be kept in
// sync.
type ServiceDefinitionConnect struct {
	// TODO(banks) add way to specify that the app is connect-native
	// Proxy configures a connect proxy instance for the service
	Proxy *ServiceDefinitionConnectProxy
}

// ServiceDefinitionConnectProxy is the connect proxy config  within a service
// registration. Note this is duplicated in config.ServiceConnectProxy and needs
// to be kept in sync.
type ServiceDefinitionConnectProxy struct {
	Command  []string
	ExecMode string
	Config   map[string]interface{}
}
