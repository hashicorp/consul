package agent

import (
	"github.com/marouenj/consul-template/core"
	"github.com/hashicorp/consul/consul/structs"
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

// ArchetypeDefinition is used to JSON decode the Archetype definitions
type ArchetypeDefinition struct {
	ID       string // identifies this definition
	PoolName string // identifies an archetype. The convention is that definitions with the same PoolName relate to the same pool, 'redis_pool' for example
	Tags     []string
	Address  string
	Port     int
	Check    CheckType           // potentially a template, to be rendered by consul-template
	Checks   CheckTypes          // ditto
	Template core.ConfigTemplate // template for the config file of the service this archetype abstracts, to be rendered by consul-template
}

func (a *ArchetypeDefinition) NodeService() *structs.NodeService {
	ns := &structs.NodeService{
		ID:      a.ID,
		Service: a.PoolName,
		Tags:    a.Tags,
		Address: a.Address,
		Port:    a.Port,
	}
	if ns.ID == "" && ns.Service != "" {
		ns.ID = ns.Service
	}
	return ns
}

func (a *ArchetypeDefinition) CheckTypes() (checks CheckTypes) {
	a.Checks = append(a.Checks, &a.Check)
	for _, check := range a.Checks {
		if check.Valid() {
			checks = append(checks, check)
		}
	}
	return
}

func (a *ArchetypeDefinition) NodeArchetype() *structs.NodeArchetype {
	na := &structs.NodeArchetype{
		ID:        a.ID,
		Archetype: a.PoolName,
		Tags:      a.Tags,
		Address:   a.Address,
		Port:      a.Port,
	}
	if na.ID == "" && na.Archetype != "" {
		na.ID = na.Archetype
	}
	return na
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
