package agent

import (
	"github.com/marouenj/consul/consul/structs"
)

// ServiceDefinition is used to JSON decode the Service definitions
type ServiceDefinition struct {
	ID      string
	Name    string
	Tags    []string
	Address string
	Port    int
	Check   CheckType
	Checks  CheckTypes
}

func (s *ServiceDefinition) NodeService() *structs.NodeService {
	ns := &structs.NodeService{
		ID:      s.ID,
		Service: s.Name,
		Tags:    s.Tags,
		Address: s.Address,
		Port:    s.Port,
	}
	if ns.ID == "" && ns.Service != "" {
		ns.ID = ns.Service
	}
	return ns
}

func (s *ServiceDefinition) CheckTypes() (checks CheckTypes) {
	s.Checks = append(s.Checks, &s.Check)
	for _, check := range s.Checks {
		if check.Valid() {
			checks = append(checks, check)
		}
	}
	return
}

// ChecKDefinition is used to JSON decode the Check definitions
type CheckDefinition struct {
	ID        string
	Name      string
	Notes     string
	ServiceID string
	CheckType `mapstructure:",squash"`
}

func (c *CheckDefinition) HealthCheck(node string) *structs.HealthCheck {
	health := &structs.HealthCheck{
		Node:      node,
		CheckID:   c.ID,
		Name:      c.Name,
		Status:    structs.HealthCritical,
		Notes:     c.Notes,
		ServiceID: c.ServiceID,
	}
	if health.CheckID == "" && health.Name != "" {
		health.CheckID = health.Name
	}
	return health
}
