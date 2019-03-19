package state

import (
	"reflect"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
)

func TestStore_ConfigEntry(t *testing.T) {
	s := testStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		ProxyConfig: structs.ConnectProxyConfig{
			DestinationServiceName: "foo",
		},
	}

	// Create
	if err := s.EnsureConfigEntry(0, expected); err != nil {
		t.Fatal(err)
	}

	{
		idx, config, err := s.ConfigEntry(structs.ProxyDefaults, "global")
		if err != nil {
			t.Fatal(err)
		}
		if idx != 0 {
			t.Fatalf("bad: %d", idx)
		}
		if !reflect.DeepEqual(expected, config) {
			t.Fatalf("bad: %#v, %#v", expected, config)
		}
	}

	// Update
	updated := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		ProxyConfig: structs.ConnectProxyConfig{
			DestinationServiceName: "bar",
		},
	}
	if err := s.EnsureConfigEntry(1, updated); err != nil {
		t.Fatal(err)
	}

	{
		idx, config, err := s.ConfigEntry(structs.ProxyDefaults, "global")
		if err != nil {
			t.Fatal(err)
		}
		if idx != 1 {
			t.Fatalf("bad: %d", idx)
		}
		if !reflect.DeepEqual(updated, config) {
			t.Fatalf("bad: %#v, %#v", updated, config)
		}
	}

	// Delete
	if err := s.DeleteConfigEntry(structs.ProxyDefaults, "global"); err != nil {
		t.Fatal(err)
	}

	_, config, err := s.ConfigEntry(structs.ProxyDefaults, "global")
	if err != nil {
		t.Fatal(err)
	}
	if config != nil {
		t.Fatalf("config should be deleted: %v", config)
	}
}
