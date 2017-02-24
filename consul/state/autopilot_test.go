package state

import (
	"reflect"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

func TestStateStore_Autopilot(t *testing.T) {
	s := testStateStore(t)

	expected := &structs.AutopilotConfig{
		DeadServerCleanup: true,
	}

	if err := s.UpdateAutopilotConfig(expected); err != nil {
		t.Fatal(err)
	}

	idx, config, err := s.AutopilotConfig()
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
