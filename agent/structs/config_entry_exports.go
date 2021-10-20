package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// ServiceExportsConfigEntry is the top-level struct for exporting a service to be exposed
// across other admin partitions.
type ServiceExportsConfigEntry struct {
	Partition string

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

func (e *ServiceExportsConfigEntry) Clone() *ServiceExportsConfigEntry {
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

func (e *ServiceExportsConfigEntry) GetKind() string {
	return ServiceExports
}

func (e *ServiceExportsConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Partition
}

func (e *ServiceExportsConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ServiceExportsConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	meta := DefaultEnterpriseMetaInPartition(e.Partition)
	e.EnterpriseMeta.Merge(meta)
	e.EnterpriseMeta.Normalize()

	for i := range e.Services {
		e.Services[i].Namespace = NamespaceOrDefault(e.Services[i].Namespace)
	}

	return nil
}

func (e *ServiceExportsConfigEntry) Validate() error {
	if e.Partition == "" {
		return fmt.Errorf("Partition is required")
	}
	if e.Partition == WildcardSpecifier {
		return fmt.Errorf("service-exports Partition must be the name of a partition, and not a wildcard")
	}

	validationErr := validateConfigEntryMeta(e.Meta)

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

	return validationErr
}

func (e *ServiceExportsConfigEntry) CanRead(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.MeshRead(&authzContext) == acl.Allow
}

func (e *ServiceExportsConfigEntry) CanWrite(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.MeshWrite(&authzContext) == acl.Allow
}

func (e *ServiceExportsConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceExportsConfigEntry) GetEnterpriseMeta() *EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}
