package state

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/pascaldekloe/goe/verify"
)

func TestStateStore_Autopilot(t *testing.T) {
	s := testStateStore(t)

	expected := &structs.AutopilotConfig{
		CleanupDeadServers:      true,
		LastContactThreshold:    5 * time.Second,
		MaxTrailingLogs:         500,
		ServerStabilizationTime: 100 * time.Second,
		RedundancyZoneTag:       "az",
		DisableUpgradeMigration: true,
		UpgradeVersionTag:       "build",
	}

	if err := s.AutopilotSetConfig(0, expected); err != nil {
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

func TestStateStore_AutopilotCAS(t *testing.T) {
	s := testStateStore(t)

	expected := &structs.AutopilotConfig{
		CleanupDeadServers: true,
	}

	if err := s.AutopilotSetConfig(0, expected); err != nil {
		t.Fatal(err)
	}
	if err := s.AutopilotSetConfig(1, expected); err != nil {
		t.Fatal(err)
	}

	// Do a CAS with an index lower than the entry
	ok, err := s.AutopilotCASConfig(2, 0, &structs.AutopilotConfig{
		CleanupDeadServers: false,
	})
	if ok || err != nil {
		t.Fatalf("expected (false, nil), got: (%v, %#v)", ok, err)
	}

	// Check that the index is untouched and the entry
	// has not been updated.
	idx, config, err := s.AutopilotConfig()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 {
		t.Fatalf("bad: %d", idx)
	}
	if !config.CleanupDeadServers {
		t.Fatalf("bad: %#v", config)
	}

	// Do another CAS, this time with the correct index
	ok, err = s.AutopilotCASConfig(2, 1, &structs.AutopilotConfig{
		CleanupDeadServers: false,
	})
	if !ok || err != nil {
		t.Fatalf("expected (true, nil), got: (%v, %#v)", ok, err)
	}

	// Make sure the config was updated
	idx, config, err = s.AutopilotConfig()
	if err != nil {
		t.Fatal(err)
	}
	if idx != 2 {
		t.Fatalf("bad: %d", idx)
	}
	if config.CleanupDeadServers {
		t.Fatalf("bad: %#v", config)
	}
}

func TestStateStore_Autopilot_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)
	before := &structs.AutopilotConfig{
		CleanupDeadServers: true,
	}
	if err := s.AutopilotSetConfig(99, before); err != nil {
		t.Fatal(err)
	}

	snap := s.Snapshot()
	defer snap.Close()

	after := &structs.AutopilotConfig{
		CleanupDeadServers: false,
	}
	if err := s.AutopilotSetConfig(100, after); err != nil {
		t.Fatal(err)
	}

	snapped, err := snap.Autopilot()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	verify.Values(t, "", before, snapped)

	s2 := testStateStore(t)
	restore := s2.Restore()
	if err := restore.Autopilot(snapped); err != nil {
		t.Fatalf("err: %s", err)
	}
	restore.Commit()

	idx, res, err := s2.AutopilotConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 99 {
		t.Fatalf("bad index: %d", idx)
	}
	verify.Values(t, "", before, res)
}
