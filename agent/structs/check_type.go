package structs

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
)

type CheckTypes []*CheckType

// CheckType is used to create either the CheckMonitor or the CheckTTL.
// The following types are supported: Script, HTTP, TCP, Docker, TTL, GRPC, Alias, H2PING. Script,
// HTTP, Docker, TCP, GRPC, and H2PING all require Interval. Only one of the types may
// to be provided: TTL or Script/Interval or HTTP/Interval or TCP/Interval or
// Docker/Interval or GRPC/Interval or AliasService or H2PING/Interval.
// Since types like CheckHTTP and CheckGRPC derive from CheckType, there are
// helper conversion methods that do the reverse conversion. ie. checkHTTP.CheckType()
type CheckType struct {
	// fields already embedded in CheckDefinition
	// Note: CheckType.CheckID == CheckDefinition.ID

	CheckID types.CheckID
	Name    string
	Status  string
	Notes   string

	// fields copied to CheckDefinition
	// Update CheckDefinition when adding fields here

	ScriptArgs             []string
	HTTP                   string
	H2PING                 string
	H2PingUseTLS           bool
	Header                 map[string][]string
	Method                 string
	Body                   string
	DisableRedirects       bool
	TCP                    string
	UDP                    string
	Interval               time.Duration
	AliasNode              string
	AliasService           string
	DockerContainerID      string
	Shell                  string
	GRPC                   string
	GRPCUseTLS             bool
	OSService              string
	TLSServerName          string
	TLSSkipVerify          bool
	Timeout                time.Duration
	TTL                    time.Duration
	SuccessBeforePassing   int
	FailuresBeforeWarning  int
	FailuresBeforeCritical int

	// Definition fields used when exposing checks through a proxy
	ProxyHTTP string
	ProxyGRPC string

	// DeregisterCriticalServiceAfter, if >0, will cause the associated
	// service, if any, to be deregistered if this check is critical for
	// longer than this duration.
	DeregisterCriticalServiceAfter time.Duration
	OutputMaxSize                  int
}

func (t *CheckType) UnmarshalJSON(data []byte) (err error) {
	type Alias CheckType
	aux := &struct {
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
		H2PingUseTLSSnake                   bool        `json:"h2ping_use_tls"`

		// These are going to be ignored but since we are disallowing unknown fields
		// during parsing we have to be explicit about parsing but not using these.
		ServiceID      string `json:"ServiceID"`
		ServiceIDSnake string `json:"service_id"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Preevaluate struct values to determine where to set defaults
	if err = lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
	// Set defaults
	if aux.H2PING != "" {
		aux.H2PingUseTLS = true
		aux.H2PingUseTLSSnake = true
	}

	if err = lib.UnmarshalJSON(data, aux); err != nil {
		return err
	}
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
	if (aux.H2PING != "" && !aux.H2PingUseTLSSnake) || (aux.H2PING == "" && aux.H2PingUseTLSSnake) {
		t.H2PingUseTLS = aux.H2PingUseTLSSnake
	}

	return nil

}

// Validate returns an error message if the check is invalid
func (c *CheckType) Validate() error {
	intervalCheck := c.IsScript() || c.HTTP != "" || c.TCP != "" || c.UDP != "" || c.GRPC != "" || c.H2PING != "" || c.OSService != ""

	if c.Interval > 0 && c.TTL > 0 {
		return fmt.Errorf("Interval and TTL cannot both be specified")
	}
	if intervalCheck && c.Interval <= 0 {
		return fmt.Errorf("Interval must be > 0 for Script, HTTP, H2PING, TCP, UDP or OSService checks")
	}
	if intervalCheck && c.IsAlias() {
		return fmt.Errorf("Interval cannot be set for Alias checks")
	}
	if c.IsAlias() && c.TTL > 0 {
		return fmt.Errorf("TTL must be not be set for Alias checks")
	}
	if !intervalCheck && !c.IsAlias() && c.TTL <= 0 {
		return fmt.Errorf("TTL must be > 0 for TTL checks")
	}
	if c.OutputMaxSize < 0 {
		return fmt.Errorf("MaxOutputMaxSize must be positive")
	}
	if c.FailuresBeforeWarning > c.FailuresBeforeCritical {
		return fmt.Errorf("FailuresBeforeWarning can't be higher than FailuresBeforeCritical")
	}

	return nil
}

// Empty checks if the CheckType has no fields defined. Empty checks parsed from json configs are filtered out
func (c *CheckType) Empty() bool {
	return reflect.DeepEqual(c, &CheckType{})
}

// IsAlias checks if this is an alias check.
func (c *CheckType) IsAlias() bool {
	return c.AliasNode != "" || c.AliasService != ""
}

// IsScript checks if this is a check that execs some kind of script.
func (c *CheckType) IsScript() bool {
	return len(c.ScriptArgs) > 0
}

// IsTTL checks if this is a TTL type
func (c *CheckType) IsTTL() bool {
	return c.TTL > 0
}

// IsMonitor checks if this is a Monitor type
func (c *CheckType) IsMonitor() bool {
	return c.IsScript() && c.DockerContainerID == "" && c.Interval > 0
}

// IsHTTP checks if this is a HTTP type
func (c *CheckType) IsHTTP() bool {
	return c.HTTP != "" && c.Interval > 0
}

// IsTCP checks if this is a TCP type
func (c *CheckType) IsTCP() bool {
	return c.TCP != "" && c.Interval > 0
}

func (c *CheckType) IsUDP() bool {
	return c.UDP != "" && c.Interval > 0
}

// IsDocker returns true when checking a docker container.
func (c *CheckType) IsDocker() bool {
	return c.IsScript() && c.DockerContainerID != "" && c.Interval > 0
}

// IsGRPC checks if this is a GRPC type
func (c *CheckType) IsGRPC() bool {
	return c.GRPC != "" && c.Interval > 0
}

// IsH2PING checks if this is a H2PING type
func (c *CheckType) IsH2PING() bool {
	return c.H2PING != "" && c.Interval > 0
}

// IsOSService checks if this is a WindowsService/systemd type
func (c *CheckType) IsOSService() bool {
	return c.OSService != "" && c.Interval > 0
}

func (c *CheckType) Type() string {
	switch {
	case c.IsGRPC():
		return "grpc"
	case c.IsHTTP():
		return "http"
	case c.IsTTL():
		return "ttl"
	case c.IsTCP():
		return "tcp"
	case c.IsUDP():
		return "udp"
	case c.IsAlias():
		return "alias"
	case c.IsDocker():
		return "docker"
	case c.IsScript():
		return "script"
	case c.IsH2PING():
		return "h2ping"
	case c.IsOSService():
		return "os_service"
	default:
		return ""
	}
}
