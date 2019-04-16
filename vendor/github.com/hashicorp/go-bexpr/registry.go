package bexpr

import (
	"reflect"
	"sync"
)

var DefaultRegistry Registry = NewSyncRegistry()

type Registry interface {
	GetFieldConfigurations(reflect.Type) (FieldConfigurations, error)
}

type SyncRegistry struct {
	configurations map[reflect.Type]FieldConfigurations
	lock           sync.RWMutex
}

func NewSyncRegistry() *SyncRegistry {
	return &SyncRegistry{
		configurations: make(map[reflect.Type]FieldConfigurations),
	}
}

func (r *SyncRegistry) GetFieldConfigurations(rtype reflect.Type) (FieldConfigurations, error) {
	if r != nil {
		r.lock.RLock()
		configurations, ok := r.configurations[rtype]
		r.lock.RUnlock()

		if ok {
			return configurations, nil
		}
	}

	fields, err := generateFieldConfigurations(rtype)
	if err != nil {
		return nil, err
	}

	if r != nil {
		r.lock.Lock()
		r.configurations[rtype] = fields
		r.lock.Unlock()
	}

	return fields, nil
}

type nilRegistry struct{}

// The pass through registry can be used to prevent using the default registry and thus storing
// any field configurations
var NilRegistry = (*nilRegistry)(nil)

func (r *nilRegistry) GetFieldConfigurations(rtype reflect.Type) (FieldConfigurations, error) {
	fields, err := generateFieldConfigurations(rtype)
	return fields, err
}
