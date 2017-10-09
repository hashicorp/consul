package structs

// ServiceDefinition is used to JSON decode the Service definitions
type ServiceDefinition struct {
	ID                string
	Name              string
	Tags              []string
	Address           string
	Port              int
	Check             CheckType
	Checks            CheckTypes
	Token             string
	EnableTagOverride bool
}

func (s *ServiceDefinition) NodeService() *NodeService {
	ns := &NodeService{
		ID:                s.ID,
		Service:           s.Name,
		Tags:              s.Tags,
		Address:           s.Address,
		Port:              s.Port,
		EnableTagOverride: s.EnableTagOverride,
	}
	if ns.ID == "" && ns.Service != "" {
		ns.ID = ns.Service
	}
	return ns
}

func (s *ServiceDefinition) CheckTypes() (checks CheckTypes, err error) {
	if !s.Check.Empty() {
		_, err := s.Check.Valid()
		if err != nil {
			return nil, err
		}
		s.Checks = append(s.Checks, &s.Check)
	}
	for _, check := range s.Checks {
		_, err := check.Valid()
		if err != nil {
			return nil, err
		}
	}
	return s.Checks, nil
}
