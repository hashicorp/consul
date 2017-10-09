package structs

import "fmt"

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
	s.Checks = append(s.Checks, &s.Check)
	for _, check := range s.Checks {
		if check.Empty() {
			continue
		}
		if check.Valid() {
			checks = append(checks, check)
		} else {
			return nil, fmt.Errorf("invalid check definition:%v", check.ValidateMsg())
		}
	}
	return checks, nil
}
