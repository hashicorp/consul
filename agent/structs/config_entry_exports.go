// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	Services []ExportedService `json:",omitempty"`

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
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
	Consumers []ServiceConsumer `json:",omitempty"`
}

// ServiceConsumer represents a downstream consumer of the service to be exported.
// At most one of Partition or Peer must be specified.
type ServiceConsumer struct {
	// Partition is the admin partition to export the service to.
	Partition string `json:",omitempty"`

	// Peer is the name of the peer to export the service to.
	Peer string `json:",omitempty" alias:"peer_name"`

	// SamenessGroup is the name of the sameness group to export the service to.
	SamenessGroup string `json:",omitempty" alias:"sameness_group"`
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
		e.Services[i].Namespace = acl.NormalizeNamespace(e.Services[i].Namespace)
	}

	return nil
}

func (e *ExportedServicesConfigEntry) Validate() error {
	if err := validateExportedServicesName(e.Name); err != nil {
		return err
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	if err := e.validateServicesEnterprise(); err != nil {
		return err
	}

	return e.validateServices()
}

func (e *ExportedServicesConfigEntry) validateServices() error {
	for i, svc := range e.Services {
		if svc.Name == "" {
			return fmt.Errorf("Services[%d]: service name cannot be empty", i)
		}
		if svc.Namespace == WildcardSpecifier && svc.Name != WildcardSpecifier {
			return fmt.Errorf("Services[%d]: service name must be wildcard if namespace is wildcard", i)
		}
		if len(svc.Consumers) == 0 {
			return fmt.Errorf("Services[%d]: must have at least one consumer", i)
		}
		for j, consumer := range svc.Consumers {
			count := 0
			if consumer.Peer != "" {
				count++
			}
			if consumer.Partition != "" {
				count++
			}
			if consumer.SamenessGroup != "" {
				count++
			}
			if count > 1 {
				return fmt.Errorf("Services[%d].Consumers[%d]: must define at most one of Peer, Partition, or SamenessGroup", i, j)
			}
			if consumer.Partition == WildcardSpecifier {
				return fmt.Errorf("Services[%d].Consumers[%d]: exporting to all partitions (wildcard) is not supported", i, j)
			}
			if consumer.Peer == WildcardSpecifier {
				return fmt.Errorf("Services[%d].Consumers[%d]: exporting to all peers (wildcard) is not supported", i, j)
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

func (e *ExportedServicesConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
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
