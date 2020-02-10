package structs

import (
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
)

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
	ScriptArgs                     []string
	HTTP                           string
	Header                         map[string][]string
	Method                         string
	Body                           string
	TCP                            string
	Interval                       time.Duration
	DockerContainerID              string
	Shell                          string
	GRPC                           string
	GRPCUseTLS                     bool
	TLSSkipVerify                  bool
	AliasNode                      string
	AliasService                   string
	Timeout                        time.Duration
	TTL                            time.Duration
	SuccessBeforePassing           int
	FailuresBeforeCritical         int
	DeregisterCriticalServiceAfter time.Duration
	OutputMaxSize                  int

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

func (t *CheckDefinition) UnmarshalJSON(data []byte) (err error) {
	type Alias CheckDefinition
	aux := &struct {
		// Parse special values
		Interval                       interface{}
		Timeout                        interface{}
		TTL                            interface{}
		DeregisterCriticalServiceAfter interface{}

		// Translate fields

		// "args" -> ScriptArgs
		Args                                []string    `json:"args"`
		ScriptArgsSnake                     []string    `json:"script_args"`
		DeregisterCriticalServiceAfterSnake interface{} `json:"deregister_critical_service_after"`
		DockerContainerIDSnake              string      `json:"docker_container_id"`
		TLSSkipVerifySnake                  bool        `json:"tls_skip_verify"`
		ServiceIDSnake                      string      `json:"service_id"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}

	// Translate Fields
	if aux.DeregisterCriticalServiceAfter == nil {
		aux.DeregisterCriticalServiceAfter = aux.DeregisterCriticalServiceAfterSnake
	}
	if len(t.ScriptArgs) == 0 {
		t.ScriptArgs = aux.Args
	}
	if len(t.ScriptArgs) == 0 {
		t.ScriptArgs = aux.ScriptArgsSnake
	}
	if t.DockerContainerID == "" {
		t.DockerContainerID = aux.DockerContainerIDSnake
	}
	if aux.TLSSkipVerifySnake {
		t.TLSSkipVerify = aux.TLSSkipVerifySnake
	}
	if t.ServiceID == "" {
		t.ServiceID = aux.ServiceIDSnake
	}

	// Parse special values
	if aux.Interval != nil {
		switch v := aux.Interval.(type) {
		case string:
			if t.Interval, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.Interval = time.Duration(v)
		}
	}
	if aux.Timeout != nil {
		switch v := aux.Timeout.(type) {
		case string:
			if t.Timeout, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.Timeout = time.Duration(v)
		}
	}
	if aux.TTL != nil {
		switch v := aux.TTL.(type) {
		case string:
			if t.TTL, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.TTL = time.Duration(v)
		}
	}
	if aux.DeregisterCriticalServiceAfter != nil {
		switch v := aux.DeregisterCriticalServiceAfter.(type) {
		case string:
			if t.DeregisterCriticalServiceAfter, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.DeregisterCriticalServiceAfter = time.Duration(v)
		}
	}

	return nil
}

func (c *CheckDefinition) HealthCheck(node string) *HealthCheck {
	health := &HealthCheck{
		Node:           node,
		CheckID:        c.ID,
		Name:           c.Name,
		Status:         api.HealthCritical,
		Notes:          c.Notes,
		ServiceID:      c.ServiceID,
		EnterpriseMeta: c.EnterpriseMeta,
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
		CheckID: c.ID,
		Name:    c.Name,
		Status:  c.Status,
		Notes:   c.Notes,

		ScriptArgs:                     c.ScriptArgs,
		AliasNode:                      c.AliasNode,
		AliasService:                   c.AliasService,
		HTTP:                           c.HTTP,
		GRPC:                           c.GRPC,
		GRPCUseTLS:                     c.GRPCUseTLS,
		Header:                         c.Header,
		Method:                         c.Method,
		Body:                           c.Body,
		OutputMaxSize:                  c.OutputMaxSize,
		TCP:                            c.TCP,
		Interval:                       c.Interval,
		DockerContainerID:              c.DockerContainerID,
		Shell:                          c.Shell,
		TLSSkipVerify:                  c.TLSSkipVerify,
		Timeout:                        c.Timeout,
		TTL:                            c.TTL,
		SuccessBeforePassing:           c.SuccessBeforePassing,
		FailuresBeforeCritical:         c.FailuresBeforeCritical,
		DeregisterCriticalServiceAfter: c.DeregisterCriticalServiceAfter,
	}
}
