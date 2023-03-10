package resource

import (
	"testing"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	r := NewRegistry()

	serviceType := &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v1",
		Kind:         "service",
	}

	// register
	serviceRegistration := Registration{Type: serviceType}
	r.Register(serviceRegistration)

	// registering again should panic
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic, but none occurred")
		} else {
			errstr, ok := r.(string)
			if !ok {
				t.Errorf("unexpected error type returned from panic")
			} else if errstr != "resource type mesh/v1/service already registered" {
				t.Errorf("unexpected error message: %s", errstr)
			}
		}
	}()
	r.Register(serviceRegistration)
}

func TestResolve(t *testing.T) {
	r := NewRegistry()

	serviceType := &pbresource.Type{
		Group:        "mesh",
		GroupVersion: "v1",
		Kind:         "service",
	}

	// not found
	_, ok := r.Resolve(serviceType)
	assert.False(t, ok)

	// found
	r.Register(Registration{Type: serviceType})
	registration, ok := r.Resolve(serviceType)
	assert.True(t, ok)
	assert.Equal(t, registration.Type, serviceType)
}
