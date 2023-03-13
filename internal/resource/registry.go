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
}

// Resource type registry
type TypeRegistry struct {
	registrations map[*pbresource.Type]Registration
	lock          sync.RWMutex
}

func NewRegistry() Registry {
	return &TypeRegistry{
		registrations: make(map[*pbresource.Type]Registration),
	}
}

func (r *TypeRegistry) Register(registration Registration) {
	r.lock.Lock()
	defer r.lock.Unlock()

	typ := registration.Type
	if typ.Group == "" || typ.GroupVersion == "" || typ.Kind == "" {
		panic("type field(s) cannot be empty")
	}

	if _, ok := r.registrations[registration.Type]; ok {
		panic(fmt.Sprintf("resource type %s already registered", ToGVK(registration.Type)))
	}

	r.registrations[registration.Type] = registration
}

func (r *TypeRegistry) Resolve(typ *pbresource.Type) (reg Registration, ok bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if registration, ok := r.registrations[typ]; ok {
		return registration, true
	}
	return Registration{}, false
}

func ToGVK(resourceType *pbresource.Type) string {
	// TODO: is `/` delimiter safe?
	return fmt.Sprintf("%s/%s/%s", resourceType.Group, resourceType.GroupVersion, resourceType.Kind)
}
