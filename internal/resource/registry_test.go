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

	// register existing should panic
	assertRegisterPanics(t, r.Register, serviceRegistration, "resource type mesh/v1/service already registered")

	// register empty Group should panic
	assertRegisterPanics(t, r.Register, Registration{
		Type: &pbresource.Type{
			Group:        "",
			GroupVersion: "v1",
			Kind:         "service",
		},
	}, "type field(s) cannot be empty")

	// register empty GroupVersion should panic
	assertRegisterPanics(t, r.Register, Registration{
		Type: &pbresource.Type{
			Group:        "mesh",
			GroupVersion: "",
			Kind:         "service",
		},
	}, "type field(s) cannot be empty")

	// register empty Kind should panic
	assertRegisterPanics(t, r.Register, Registration{
		Type: &pbresource.Type{
			Group:        "mesh",
			GroupVersion: "v1",
			Kind:         "",
		},
	}, "type field(s) cannot be empty")
}

func assertRegisterPanics(t *testing.T, registerFn func(reg Registration), registration Registration, panicString string) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic, but none occurred")
		} else {
			errstr, ok := r.(string)
			if !ok {
				t.Errorf("unexpected error type returned from panic")
			} else if errstr != panicString {
				t.Errorf("expected %s error message but got: %s", panicString, errstr)
			}
		}
	}()

	registerFn(registration)
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
