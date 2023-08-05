// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"time"

	"github.com/hashicorp/consul/acl"

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
	H2PING                         string
	H2PingUseTLS                   bool
	Header                         map[string][]string
	Method                         string
	Body                           string
	DisableRedirects               bool
	TCP                            string
	UDP                            string
	Interval                       time.Duration
	DockerContainerID              string
	Shell                          string
	GRPC                           string
	GRPCUseTLS                     bool
	OSService                      string
	TLSServerName                  string
	TLSSkipVerify                  bool
	AliasNode                      string
	AliasService                   string
	Timeout                        time.Duration
	TTL                            time.Duration
	SuccessBeforePassing           int
	FailuresBeforeWarning          int
	FailuresBeforeCritical         int
	DeregisterCriticalServiceAfter time.Duration
	OutputMaxSize                  int

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
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
		TLSServerNameSnake                  string      `json:"tls_server_name"`
		TLSSkipVerifySnake                  bool        `json:"tls_skip_verify"`
		GRPCUseTLSSnake                     bool        `json:"grpc_use_tls"`
		ServiceIDSnake                      string      `json:"service_id"`
		H2PingUseTLSSnake                   bool        `json:"h2ping_use_tls"`
		DisableRedirectsSnake               bool        `json:"disable_redirects"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Preevaluate struct values to determine where to set defaults
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	// Set defaults
	if aux.H2PING != "" {
		aux.H2PingUseTLS = true
		aux.H2PingUseTLSSnake = true
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
	if t.TLSServerName == "" {
		t.TLSServerName = aux.TLSServerNameSnake
	}
	if aux.TLSSkipVerifySnake {
		t.TLSSkipVerify = aux.TLSSkipVerifySnake
	}
	if aux.GRPCUseTLSSnake {
		t.GRPCUseTLS = aux.GRPCUseTLSSnake
	}
	if t.ServiceID == "" {
		t.ServiceID = aux.ServiceIDSnake
	}
	if aux.DisableRedirectsSnake {
		t.DisableRedirects = aux.DisableRedirectsSnake
	}

	if (aux.H2PING != "" && !aux.H2PingUseTLSSnake) || (aux.H2PING == "" && aux.H2PingUseTLSSnake) {
		t.H2PingUseTLS = aux.H2PingUseTLSSnake
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
		Interval:       c.Interval.String(),
		Timeout:        c.Timeout.String(),
		TTL:            c.TTL.String(),
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
		H2PING:                         c.H2PING,
		H2PingUseTLS:                   c.H2PingUseTLS,
		GRPC:                           c.GRPC,
		GRPCUseTLS:                     c.GRPCUseTLS,
		Header:                         c.Header,
		Method:                         c.Method,
		Body:                           c.Body,
		DisableRedirects:               c.DisableRedirects,
		OutputMaxSize:                  c.OutputMaxSize,
		TCP:                            c.TCP,
		UDP:                            c.UDP,
		Interval:                       c.Interval,
		DockerContainerID:              c.DockerContainerID,
		Shell:                          c.Shell,
		OSService:                      c.OSService,
		TLSServerName:                  c.TLSServerName,
		TLSSkipVerify:                  c.TLSSkipVerify,
		Timeout:                        c.Timeout,
		TTL:                            c.TTL,
		SuccessBeforePassing:           c.SuccessBeforePassing,
		FailuresBeforeWarning:          c.FailuresBeforeWarning,
		FailuresBeforeCritical:         c.FailuresBeforeCritical,
		DeregisterCriticalServiceAfter: c.DeregisterCriticalServiceAfter,
	}
}
