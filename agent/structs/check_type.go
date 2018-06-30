package structs

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/consul/types"
)

// CheckType is used to create either the CheckMonitor or the CheckTTL.
// The following types are supported: Script, HTTP, TCP, Docker, TTL, GRPC, Alias. Script,
// HTTP, Docker, TCP and GRPC all require Interval. Only one of the types may
// to be provided: TTL or Script/Interval or HTTP/Interval or TCP/Interval or
// Docker/Interval or GRPC/Interval or AliasService.
type CheckType struct {
	// fields already embedded in CheckDefinition
	// Note: CheckType.CheckID == CheckDefinition.ID

	CheckID types.CheckID
	Name    string
	Status  string
	Notes   string

	// fields copied to CheckDefinition
	// Update CheckDefinition when adding fields here

	ScriptArgs        []string
	HTTP              string
	Header            map[string][]string
	Method            string
	TCP               string
	Interval          time.Duration
	AliasNode         string
	AliasService      string
	DockerContainerID string
	Shell             string
	GRPC              string
	GRPCUseTLS        bool
	TLSSkipVerify     bool
	Timeout           time.Duration
	TTL               time.Duration

	// DeregisterCriticalServiceAfter, if >0, will cause the associated
	// service, if any, to be deregistered if this check is critical for
	// longer than this duration.
	DeregisterCriticalServiceAfter time.Duration
}
type CheckTypes []*CheckType

// Validate returns an error message if the check is invalid
func (c *CheckType) Validate() error {
	intervalCheck := c.IsScript() || c.HTTP != "" || c.TCP != "" || c.GRPC != ""

	if c.Interval > 0 && c.TTL > 0 {
		return fmt.Errorf("Interval and TTL cannot both be specified")
	}
	if intervalCheck && c.Interval <= 0 {
		return fmt.Errorf("Interval must be > 0 for Script, HTTP, or TCP checks")
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

// IsDocker returns true when checking a docker container.
func (c *CheckType) IsDocker() bool {
	return c.IsScript() && c.DockerContainerID != "" && c.Interval > 0
}

// IsGRPC checks if this is a GRPC type
func (c *CheckType) IsGRPC() bool {
	return c.GRPC != "" && c.Interval > 0
}
