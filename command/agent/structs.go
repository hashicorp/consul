package agent

import (
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/types"
)

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

func (s *ServiceDefinition) NodeService() *structs.NodeService {
	ns := &structs.NodeService{
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

func (s *ServiceDefinition) CheckTypes() (checks CheckTypes) {
	s.Checks = append(s.Checks, &s.Check)
	for _, check := range s.Checks {
		if check.Valid() {
			checks = append(checks, check)
		}
	}
	return
}

// CheckDefinition is used to JSON decode the Check definitions
type CheckDefinition struct {
	ID        types.CheckID
	Name      string
	Notes     string
	ServiceID string
	Token     string
	Status    string

	// Copied fields from CheckType without the fields
	// already present in CheckDefinition:
	//
	//   ID (CheckID), Name, Status, Notes
	//
	Script                         string
	HTTP                           string
	TCP                            string
	Interval                       time.Duration
	DockerContainerID              string
	Shell                          string
	TLSSkipVerify                  bool
	Timeout                        time.Duration
	TTL                            time.Duration
	DeregisterCriticalServiceAfter time.Duration
}

func (c *CheckDefinition) HealthCheck(node string) *structs.HealthCheck {
	health := &structs.HealthCheck{
		Node:      node,
		CheckID:   c.ID,
		Name:      c.Name,
		Status:    api.HealthCritical,
		Notes:     c.Notes,
		ServiceID: c.ServiceID,
	}
	if c.Status != "" {
		health.Status = c.Status
	}
	if health.CheckID == "" && health.Name != "" {
		health.CheckID = types.CheckID(health.Name)
	}
	return health
}

func (c *CheckDefinition) CheckType() *CheckType {
	return &CheckType{
		CheckID:           c.ID,
		Name:              c.Name,
		Script:            c.Script,
		HTTP:              c.HTTP,
		TCP:               c.TCP,
		Interval:          c.Interval,
		DockerContainerID: c.DockerContainerID,
		Shell:             c.Shell,
		TLSSkipVerify:     c.TLSSkipVerify,
		Timeout:           c.Timeout,
		TTL:               c.TTL,
		DeregisterCriticalServiceAfter: c.DeregisterCriticalServiceAfter,
		Status: c.Status,
		Notes:  c.Notes,
	}
}

// persistedService is used to wrap a service definition and bundle it
// with an ACL token so we can restore both at a later agent start.
type persistedService struct {
	Token   string
	Service *structs.NodeService
}
