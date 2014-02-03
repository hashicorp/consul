package agent

import (
	"github.com/hashicorp/consul/consul/structs"
)

// ServiceDefinition is used to JSON decode the Service definitions
type ServiceDefinition struct {
	ID    string
	Name  string
	Tag   string
	Port  int
	Check CheckType
}

func (s *ServiceDefinition) NodeService() *structs.NodeService {
	ns := &structs.NodeService{
		ID:      s.ID,
		Service: s.Name,
		Tag:     s.Tag,
		Port:    s.Port,
	}
	if ns.ID == "" && ns.Service != "" {
		ns.ID = ns.Service
	}
	return ns
}

func (s *ServiceDefinition) CheckType() *CheckType {
	if s.Check.Script == "" && s.Check.Interval == 0 && s.Check.TTL == 0 {
		return nil
	}
	return &s.Check
}

// ChecKDefinition is used to JSON decode the Check definitions
type CheckDefinition struct {
	ID        string
	Name      string
	Notes     string
	CheckType `mapstructure:",squash"`
}

func (c *CheckDefinition) HealthCheck(node string) *structs.HealthCheck {
	health := &structs.HealthCheck{
		Node:    node,
		CheckID: c.ID,
		Name:    c.Name,
		Status:  structs.HealthUnknown,
		Notes:   c.Notes,
	}
	if health.CheckID == "" && health.Name != "" {
		health.CheckID = health.Name
	}
	return health
}
