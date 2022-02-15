package structs

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// ExportedServicesConfigEntry is the top-level struct for exporting a service to be exposed
// across other admin partitions.
type ExportedServicesConfigEntry struct {
	Name string

	// Services is a list of services to be exported and the list of partitions
	// to expose them to.
	Services []ExportedService

	Meta           map[string]string `json:",omitempty"`
	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

// ExportedService manages the exporting of a service in the local partition to
// other partitions.
type ExportedService struct {
	// Name is the name of the service to be exported.
	Name string

	// Namespace is the namespace to export the service from.
	Namespace string `json:",omitempty"`

	// Consumers is a list of downstream consumers of the service to be exported.
	Consumers []ServiceConsumer
}

// ServiceConsumer represents a downstream consumer of the service to be exported.
type ServiceConsumer struct {
	// Partition is the admin partition to export the service to.
	Partition string
}

func (e *ExportedServicesConfigEntry) ToMap() map[string]map[string][]string {
	resp := make(map[string]map[string][]string)
	for _, svc := range e.Services {
		if _, ok := resp[svc.Namespace]; !ok {
			resp[svc.Namespace] = make(map[string][]string)
		}
		if _, ok := resp[svc.Namespace][svc.Name]; !ok {
			consumers := make([]string, 0, len(svc.Consumers))
			for _, c := range svc.Consumers {
				consumers = append(consumers, c.Partition)
			}
			resp[svc.Namespace][svc.Name] = consumers
		}
	}
	return resp
}

func (e *ExportedServicesConfigEntry) Clone() *ExportedServicesConfigEntry {
	e2 := *e
	e2.Services = make([]ExportedService, len(e.Services))
	for _, svc := range e.Services {
		exportedSvc := svc
		exportedSvc.Consumers = make([]ServiceConsumer, len(svc.Consumers))
		for _, consumer := range svc.Consumers {
			exportedSvc.Consumers = append(exportedSvc.Consumers, consumer)
		}
		e2.Services = append(e2.Services, exportedSvc)
	}

	return &e2
}

func (e *ExportedServicesConfigEntry) GetKind() string {
	return ExportedServices
}

func (e *ExportedServicesConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *ExportedServicesConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ExportedServicesConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}
	e.EnterpriseMeta = *DefaultEnterpriseMetaInPartition(e.Name)
	e.EnterpriseMeta.Normalize()

	for i := range e.Services {
		e.Services[i].Namespace = NamespaceOrDefault(e.Services[i].Namespace)
	}

	return nil
}

func (e *ExportedServicesConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}
	if e.Name == WildcardSpecifier {
		return fmt.Errorf("exported-services Name must be the name of a partition, and not a wildcard")
	}

	if err := requireEnterprise(e.GetKind()); err != nil {
		return err
	}
	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	for _, svc := range e.Services {
		if svc.Name == "" {
			return fmt.Errorf("service name cannot be empty")
		}
		if len(svc.Consumers) == 0 {
			return fmt.Errorf("service %q must have at least one consumer", svc.Name)
		}
		for _, consumer := range svc.Consumers {
			if consumer.Partition == WildcardSpecifier {
				return fmt.Errorf("exporting to all partitions (wildcard) is not yet supported")
			}
		}
	}
	return nil
}

func (e *ExportedServicesConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *ExportedServicesConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *ExportedServicesConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ExportedServicesConfigEntry) GetEnterpriseMeta() *EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

// MarshalJSON adds the Kind field so that the JSON can be decoded back into the
// correct type.
// This method is implemented on the structs type (as apposed to the api type)
// because that is what the API currently uses to return a response.
func (e *ExportedServicesConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias ExportedServicesConfigEntry
	source := &struct {
		Kind string
		*Alias
	}{
		Kind:  ExportedServices,
		Alias: (*Alias)(e),
	}
	return json.Marshal(source)
}
