package resource

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Registry interface {
	// Register the given resource type and its hooks.
	Register(reg Registration)

	// Resolve the given resource type and its hooks.
	Resolve(typ *pbresource.Type) (reg Registration, ok bool)
}

type Registration struct {
	// Type is the GVK of the resource type.
	Type *pbresource.Type

	// In the future, we'll add hooks, the controller etc. here.
	// TODO: https://github.com/hashicorp/consul/pull/16622#discussion_r1134515909
}

// Hashable key for a resource type
type TypeKey string

// Resource type registry
type TypeRegistry struct {
	// registrations keyed by GVK
	registrations map[string]Registration
	lock          sync.RWMutex
}

func NewRegistry() Registry {
	return &TypeRegistry{
		registrations: make(map[string]Registration),
	}
}

func (r *TypeRegistry) Register(registration Registration) {
	r.lock.Lock()
	defer r.lock.Unlock()

	typ := registration.Type
	if typ.Group == "" || typ.GroupVersion == "" || typ.Kind == "" {
		panic("type field(s) cannot be empty")
	}

	key := ToGVK(registration.Type)
	if _, ok := r.registrations[key]; ok {
		panic(fmt.Sprintf("resource type %s already registered", key))
	}

	r.registrations[key] = registration
}

func (r *TypeRegistry) Resolve(typ *pbresource.Type) (reg Registration, ok bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if registration, ok := r.registrations[ToGVK(typ)]; ok {
		return registration, true
	}
	return Registration{}, false
}

func ToGVK(resourceType *pbresource.Type) string {
	return fmt.Sprintf("%s/%s/%s", resourceType.Group, resourceType.GroupVersion, resourceType.Kind)
}
